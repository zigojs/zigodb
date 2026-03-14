// ============================================================
// Zigo-DB: Log-Structured Database with Dual Write Regions
// Main implementation using types from types.zig
// Compatible with Zig 0.16+
// ============================================================

const std = @import("std");
const builtin = @import("builtin");
const T = @import("types.zig");

// ============================================================
// db_reserve_slot - Atomic slot reservation for multi-core safety
// ============================================================
pub fn db_reserve_slot(db: *T.ZigoDB) T.SlotReservation {
    const active_region = db.getActiveRegion();

    // Atomic fetch-add to get unique slot index - impossible to collide
    const my_idx = @atomicRmw(u32, &active_region.cursor, .Add, 1, .monotonic);

    if (my_idx >= T.MAX_ENTRIES_PER_REGION) {
        // Slot out of bounds - region is full
        return T.SlotReservation{ .slot_index = 0, .region = undefined, .success = false };
    }

    return T.SlotReservation{ .slot_index = my_idx, .region = undefined, .success = true };
}

// ============================================================
// db_write_raw - THE LOCKER WRITER (Hot Path)
// Single atomicAdd on cursor, tracks active writers for safe drain
// ============================================================
export fn db_write_raw(room_id: u32, payload_ptr: [*]const u8, payload_len: u32) i32 {
    const db = global_db orelse return -2;
    const region = db.getActiveRegion();

    // 1. Pega a ficha pra entrar na sala (increment active_writers)
    _ = @atomicRmw(i32, &region.active_writers, .Add, 1, .monotonic);

    // 2. Aumenta o counter (pega o slot)
    const idx = @atomicRmw(u32, &region.cursor, .Add, 1, .monotonic);

    // 3. Verifica se estourou o limite da metralhadora
    if (idx >= db.max_slots) {
        // Rollback - region is full
        _ = @atomicRmw(u32, &region.cursor, .Sub, 1, .monotonic);
        _ = @atomicRmw(i32, &region.active_writers, .Add, -1, .release);
        return -1; // SIGNAL: Hora de dar o switch!
    }

    // 4. Dispara a escrita no slot fixo (1KB)
    var entry = &region.entries[idx];
    entry.room_id = room_id;
    entry.timestamp = 0; // Timestamp set by Go side
    entry.data_len = if (payload_len > 992) 992 else payload_len;
    @memcpy(entry.json_data[0..entry.data_len], payload_ptr[0..entry.data_len]);

    // 5. Devolve a ficha (decrement active_writers)
    _ = @atomicRmw(i32, &region.active_writers, .Add, -1, .release);
    return 0;
}

// ============================================================
// db_release_slot - No-op now (not using active_writers)
// ============================================================
pub fn db_release_slot(db: *T.ZigoDB, region: *T.Region) void {
    _ = db;
    _ = region;
    // No active_writers tracking anymore
}

// ============================================================
// db_fill_entry
// ============================================================
pub fn db_fill_entry(entry: *T.MessageEntry, room_id: u32, uuid: u64, timestamp: i64, prev_hash: u64, state_hash: u64, data: []const u8) void {
    entry.setData(room_id, uuid, timestamp, prev_hash, state_hash, data);
    entry.checksum = entry.calculateChecksum();
}

// ============================================================
// db_needs_drain
// ============================================================
pub fn db_needs_drain(db: *T.ZigoDB) bool {
    const active_region = db.getActiveRegion();
    const cursor = @atomicLoad(u32, &active_region.cursor, .acquire);
    return cursor >= (T.MAX_ENTRIES_PER_REGION * 90) / 100;
}

// ============================================================
// db_switch_and_drain - Switch to other region and drain
// Uses active_writers for deterministic drain
// ============================================================
pub fn db_switch_and_drain(db: *T.ZigoDB) T.DrainResult {
    const current_bit = @atomicLoad(u32, &db.region_bit, .acquire);
    const new_bit = 1 - current_bit;

    // 1. Switch first - new writes go to other region
    @atomicStore(u32, &db.region_bit, new_bit, .release);

    const old_region = if (current_bit == 0) db.rw_a else db.rw_b;

    // 2. SPIN until all writers finish (deterministic drain)
    // This ensures we don't dump while writers are still writing
    var spin_count: u32 = 0;
    while (@atomicLoad(i32, &old_region.active_writers, .acquire) > 0) {
        spin_count += 1;
        if (spin_count > 10000) {
            std.Thread.yield() catch {};
            spin_count = 0;
        }
    }

    // 3. Now safe to dump
    const entries_written = @atomicLoad(u32, &old_region.cursor, .acquire);
    db.total_entries += entries_written;
    db.total_drains += 1;
    old_region.reset();

    return T.DrainResult{ .old_region = old_region, .entries_written = entries_written, .success = true };
}

// ============================================================
// db_validate_drain
// ============================================================
pub fn db_validate_drain(region: *const T.Region) bool {
    return region.cursor > 0;
}

// ============================================================
// db_should_drain
// ============================================================
pub fn db_should_drain(db: *T.ZigoDB, trigger: T.DrainTrigger, current_time: i64) bool {
    switch (trigger) {
        .SizeThreshold => return db_needs_drain(db),
        .CountThreshold => return db.getActiveRegion().cursor >= 10000,
        .TimeThreshold => return current_time - db.last_dump_time >= 30 * 1000,
        .Forced => return true,
    }
}

// ============================================================
// dump_region - Simplified to write to buffer
// ============================================================
pub fn dump_region(region: *const T.Region, buf: *std.ArrayList(u8)) !void {
    try buf.appendSlice("ZIGO_REGION_V1");
    for (region.entries[0..region.cursor]) |entry| {
        try buf.appendSlice(std.mem.asBytes(&entry.uuid));
        try buf.appendSlice(std.mem.asBytes(&entry.timestamp));
        try buf.appendSlice(std.mem.asBytes(&entry.prev_hash));
        try buf.appendSlice(std.mem.asBytes(&entry.data_len));
        try buf.appendSlice(std.mem.asBytes(&entry.checksum));
        try buf.appendSlice(entry.json_data[0..entry.data_len]);
    }
}

// ============================================================
// dumpRegionToFile - Write region to binary file
// ============================================================
pub fn dumpRegionToFile(region: *const T.Region, filename: []const u8) bool {
    _ = region;
    _ = filename;
    return false; // Implemented in Go bridge
}

// ============================================================
// dumpIndexToFile - Write index to file
// ============================================================
pub fn dumpIndexToFile(region: *const T.Region, filename: []const u8) bool {
    _ = region;
    _ = filename;
    return false; // Implemented in Go bridge
}

// ============================================================
// generateChunkFilename
// ============================================================
pub fn generateChunkFilename(timestamp: i64, initial_uuid: u64, buf: []u8) []u8 {
    const seconds: i64 = @divFloor(timestamp, 1000);
    const days: i64 = @divFloor(seconds, 86400);
    var year: i64 = 1970;
    var remaining = days;

    while (true) {
        const days_in_year: i64 = if (@mod(year, 4) == 0 and (@mod(year, 100) != 0 or @mod(year, 400) == 0)) 366 else 365;
        if (remaining < days_in_year) break;
        remaining -= days_in_year;
        year += 1;
    }

    var month: i64 = 1;
    while (month <= 12) {
        var dim: i64 = 31;
        if (month == 2) {
            dim = if (@mod(year, 4) == 0 and (@mod(year, 100) != 0 or @mod(year, 400) == 0)) 29 else 28;
        } else if (month == 4 or month == 6 or month == 9 or month == 11) {
            dim = 30;
        }
        if (remaining < dim) break;
        remaining -= dim;
        month += 1;
    }

    const day = remaining + 1;
    const day_sec = @mod(seconds, 86400);
    const hour = @divFloor(day_sec, 3600);
    const minute = @divFloor(@mod(day_sec, 3600), 60);
    const second = @mod(day_sec, 60);

    return std.fmt.bufPrint(buf, "chunk_{d:0>4}{d:0>2}{d:0>2}_{d:0>2}{d:0>2}{d:0>2}_{x}.rb", .{ year, month, day, hour, minute, second, initial_uuid }) catch unreachable;
}

// ============================================================
// Search Pool
// ============================================================
var global_search_pool: T.SearchPool = undefined;

pub fn search_init() void {
    global_search_pool = T.SearchPool.init();
}

pub fn search_reserve_segment(chunk_id: u32, data: []const u8) ?u32 {
    for (0..T.MAX_SEGMENTS) |i| {
        if (!global_search_pool.segments[i].in_use) {
            const bi = i / 64;
            const bo: u6 = @intCast(i % 64);
            const mask: u64 = @as(u64, 1) << bo;
            const old = @atomicRmw(u64, &global_search_pool.bitmap[bi], .Xchg, mask, .acquire);
            if ((old & mask) == 0) {
                global_search_pool.segments[i].chunk_id = chunk_id;
                global_search_pool.segments[i].in_use = true;
                if (data.len > 0) @memcpy(global_search_pool.segments[i].data[0..data.len], data);
                return @intCast(i);
            }
        }
    }
    return null;
}

pub fn search_release_segment(segment_id: u32) void {
    if (segment_id >= T.MAX_SEGMENTS) return;
    global_search_pool.segments[segment_id].in_use = false;
    const bi = segment_id / 64;
    const bo: u6 = @intCast(segment_id % 64);
    const mask: u64 = @as(u64, 1) << bo;
    _ = @atomicRmw(u64, &global_search_pool.bitmap[bi], .Xchg, mask, .release);
}

pub fn search_query(chunk_id: u32, query: []const u8, results: []T.MessageEntry, max_results: usize) usize {
    _ = chunk_id;
    _ = query;
    _ = results;
    _ = max_results;
    return 0;
}

// ============================================================
// CGO Export Functions
// ============================================================
var global_db: ?*T.ZigoDB = null;

export fn db_init() ?*T.ZigoDB {
    const allocator = std.heap.c_allocator;
    global_db = T.ZigoDB.init(allocator) catch null;
    search_init();
    return global_db;
}

export fn db_reserve_slot_cgo() T.SlotReservation {
    if (global_db) |db| return db_reserve_slot(db);
    return T.SlotReservation{ .slot_index = 0, .region = undefined, .success = false };
}

export fn db_release_slot_cgo() void {
    if (global_db) |db| db_release_slot(db, db.getActiveRegion());
}

export fn db_fill_entry_cgo(slot_index: u32, room_id: u32, uuid: u64, timestamp: i64, prev_hash: u64, state_hash: u64, data: [*]const u8, data_len: usize) void {
    if (global_db) |db| {
        const r = db.getActiveRegion();
        if (slot_index < r.entries.len) db_fill_entry(&r.entries[slot_index], room_id, uuid, timestamp, prev_hash, state_hash, data[0..data_len]);
    }
}

export fn db_switch_and_drain_cgo() T.DrainResult {
    if (global_db) |db| return db_switch_and_drain(db);
    return T.DrainResult{ .old_region = undefined, .entries_written = 0, .success = false };
}

// The LOCKER WRITER - single atomic call for maximum performance
export fn db_write_raw_cgo(room_id: u32, payload_ptr: [*]const u8, payload_len: u32) i32 {
    if (global_db) |db| {
        const region = db.getActiveRegion();

        // 1. Pega a ficha pra entrar na sala
        _ = @atomicRmw(i32, &region.active_writers, .Add, 1, .monotonic);

        // 2. Aumenta o counter (pega o slot)
        const idx = @atomicRmw(u32, &region.cursor, .Add, 1, .monotonic);

        // 3. Verifica se estourou o limite
        if (idx >= db.max_slots) {
            _ = @atomicRmw(u32, &region.cursor, .Sub, 1, .monotonic);
            _ = @atomicRmw(i32, &region.active_writers, .Add, -1, .release);
            return -1; // Region full
        }

        // 4. Escreve no slot
        var entry = &region.entries[idx];
        entry.room_id = room_id;
        entry.timestamp = 0; // Timestamp set by Go side
        entry.data_len = if (payload_len > 992) 992 else payload_len;
        @memcpy(entry.json_data[0..entry.data_len], payload_ptr[0..entry.data_len]);

        // 5. Devolve a ficha
        _ = @atomicRmw(i32, &region.active_writers, .Add, -1, .release);
        return 0;
    }
    return -2; // DB not initialized
}

export fn db_needs_drain_cgo() bool {
    if (global_db) |db| return db_needs_drain(db);
    return false;
}

export fn db_get_cursor_cgo() u32 {
    if (global_db) |db| {
        const a = db.getActiveRegion();
        return a.cursor;
    }
    return 0;
}

// Get last entry data for ReadLast - returns pointer to entry or null
export fn db_get_last_entry_cgo() ?*T.MessageEntry {
    if (global_db) |db| {
        const a = db.getActiveRegion();
        if (a.cursor > 0) {
            return &a.entries[a.cursor - 1];
        }
    }
    return null;
}

// Get entry by index
export fn db_get_entry_cgo(index: u32) ?*T.MessageEntry {
    if (global_db) |db| {
        const a = db.getActiveRegion();
        if (index < a.cursor) {
            return &a.entries[index];
        }
    }
    return null;
}

export fn db_dump_cgo(filepath: [*]const u8, filepath_len: usize) bool {
    if (global_db) |db| {
        const old_region = if (@atomicLoad(u32, &db.region_bit, .acquire) == 0) db.rw_b else db.rw_a;
        const filename = filepath[0..filepath_len];
        return dumpRegionToFile(old_region, filename);
    }
    return false;
}

export fn db_dump_index_cgo(filepath: [*]const u8, filepath_len: usize) bool {
    if (global_db) |db| {
        const old_region = if (@atomicLoad(u32, &db.region_bit, .acquire) == 0) db.rw_b else db.rw_a;
        const filename = filepath[0..filepath_len];
        return dumpIndexToFile(old_region, filename);
    }
    return false;
}

export fn db_get_status_cgo() u64 {
    if (global_db) |db| {
        const a = db.getActiveRegion();
        return @as(u64, a.cursor) | (@as(u64, db.total_entries) << 32);
    }
    return 0;
}

export fn search_reserve_segment_cgo(chunk_id: u32, data: [*]const u8, data_len: usize) u32 {
    return search_reserve_segment(chunk_id, data[0..data_len]) orelse T.MAX_SEGMENTS;
}

export fn search_release_segment_cgo(segment_id: u32) void {
    search_release_segment(segment_id);
}

export fn search_query_cgo(chunk_id: u32, query: [*]const u8, query_len: usize, results: [*]T.MessageEntry, max_results: usize) usize {
    return search_query(chunk_id, query[0..query_len], results[0..max_results], max_results);
}

// ============================================================
// Recovery functions
// ============================================================
pub fn db_verify_entry(entry: *const T.MessageEntry) bool {
    return entry.isValid();
}

pub fn db_verify_region(region: *const T.Region) bool {
    for (region.entries[0..region.cursor]) |*e| if (!db_verify_entry(e)) return false;
    return true;
}

pub fn db_recover(db: *T.ZigoDB) bool {
    const valid_a = db_verify_region(db.rw_a);
    const valid_b = db_verify_region(db.rw_b);
    if (valid_a and !valid_b) @atomicStore(u32, &db.region_bit, 0, .release) else if (!valid_a and valid_b) @atomicStore(u32, &db.region_bit, 1, .release);
    return valid_a or valid_b;
}

// ============================================================
// shutdown
// ============================================================
export fn db_shutdown() void {
    if (global_db) |db| {
        // Store allocator before deinit
        const allocator = std.heap.c_allocator;

        // Call deinit to clean up memory
        db.deinit(allocator);

        // Clear global_db after deinit is complete
        global_db = null;
    }
}

// ============================================================
// Replication functions
// ============================================================
pub fn generateGlobalEventId(timestamp_ns: i64, node_id: u16) u64 {
    return (@as(u64, @bitCast(timestamp_ns)) << 16) | node_id;
}

pub fn verifyHashChain(entry: *const T.MessageEntry, prev: u64) bool {
    return entry.prev_hash == prev;
}

pub fn calculateEntryHash(entry: *const T.MessageEntry) u64 {
    var h: u64 = entry.prev_hash;
    h +%= entry.uuid;
    h +%= @as(u64, @bitCast(entry.timestamp));
    for (entry.json_data[0..entry.data_len]) |b| h +%= b;
    return h;
}

pub fn db_gossip_create(chunk_id: u32, entry: *const T.MessageEntry, node_id: u16) T.GossipMessage {
    return .{ .chunk_id = chunk_id, .latest_hash = calculateEntryHash(entry), .latest_timestamp = entry.timestamp, .node_id = node_id };
}

pub fn db_gossip_verify(local: T.GossipMessage, remote: T.GossipMessage) bool {
    if (local.latest_timestamp < remote.latest_timestamp) return true else if (local.latest_timestamp == remote.latest_timestamp) return local.latest_hash == remote.latest_hash;
    return false;
}

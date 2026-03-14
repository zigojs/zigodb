// ============================================================
// Zigo-DB Types Module
// Contains all type definitions for Zig 0.16+ compatibility
// ============================================================

const std = @import("std");

// ============================================================
// Constants
// ============================================================
pub const REGION_SIZE = 32 * 1024 * 1024; // 32MB per region
pub const MAX_ENTRIES_PER_REGION = 31250; // 32MB / ~1024 bytes per entry
pub const PAGE_SIZE = 64 * 1024; // 64KB page size
pub const SEGMENT_SIZE = 64 * 1024; // 64KB per search segment
pub const MAX_SEGMENTS = 1024;

// ============================================================
// MessageEntry - Core log entry structure
// Using extern struct for CGO compatibility
// ============================================================
pub const MessageEntry = extern struct {
    room_id: u32 align(1),
    uuid: u64 align(1),
    timestamp: i64 align(1),
    prev_hash: u64 align(1),
    state_hash: u64 align(1),
    data_len: u32 align(1),
    checksum: u32 align(1),
    json_data: [1024]u8 align(1),

    pub fn init() MessageEntry {
        return MessageEntry{
            .room_id = 0,
            .uuid = 0,
            .timestamp = 0,
            .prev_hash = 0,
            .state_hash = 0,
            .data_len = 0,
            .checksum = 0,
            .json_data = undefined,
        };
    }

    pub fn setData(self: *MessageEntry, room_id: u32, uuid: u64, timestamp: i64, prev_hash: u64, state_hash: u64, data: []const u8) void {
        self.room_id = room_id;
        self.uuid = uuid;
        self.timestamp = timestamp;
        self.prev_hash = prev_hash;
        self.state_hash = state_hash;
        self.data_len = @as(u32, @intCast(@min(data.len, self.json_data.len)));
        @memcpy(self.json_data[0..self.data_len], data[0..self.data_len]);
        self.checksum = self.calculateChecksum();
    }

    pub fn calculateChecksum(self: *MessageEntry) u32 {
        var hash: u32 = 0;
        const bytes = std.mem.asBytes(self);
        for (bytes) |b| {
            hash = hash *% 31 + b;
        }
        return hash;
    }

    pub fn isValid(self: *MessageEntry) bool {
        return self.checksum == self.calculateChecksum();
    }
};

// ============================================================
// Region - Write region structure
// ============================================================
pub const Region = struct {
    cursor: u32 align(64), // Aligned to 64 bytes to avoid False Sharing
    active_writers: i32 align(64), // Aligned to 64 bytes to avoid False Sharing
    entries: []MessageEntry,
    base_ptr: ?*anyopaque,
    allocated_slice: []u8,

    pub fn init(allocator: std.mem.Allocator, size: usize) !*Region {
        const self = try allocator.create(Region);
        self.allocated_slice = try allocator.alloc(u8, size);
        self.base_ptr = self.allocated_slice.ptr;
        self.entries = try allocator.alloc(MessageEntry, MAX_ENTRIES_PER_REGION);

        for (self.entries) |*entry| {
            entry.* = MessageEntry.init();
        }

        self.cursor = 0;
        self.active_writers = 0;
        return self;
    }

    pub fn isFull(self: *const Region) bool {
        return self.cursor >= MAX_ENTRIES_PER_REGION;
    }

    pub fn isEmpty(self: *const Region) bool {
        return self.cursor == 0;
    }

    pub fn reset(self: *Region) void {
        @atomicStore(u32, &self.cursor, 0, .monotonic);
        @atomicStore(i32, &self.active_writers, 0, .monotonic);
    }

    pub fn deinit(self: *Region, allocator: std.mem.Allocator) void {
        allocator.free(self.allocated_slice);
        allocator.free(self.entries);
        allocator.destroy(self);
    }
};

// ============================================================
// ZigoDB - Main database structure
// ============================================================
pub const ZigoDB = struct {
    rw_a: *Region,
    rw_b: *Region,
    region_bit: u32 align(64), // Aligned to 64 bytes for atomic operations
    total_entries: u64,
    total_drains: u64,
    last_dump_time: i64,
    max_slots: u32,

    pub fn init(allocator: std.mem.Allocator) !*ZigoDB {
        const self = try allocator.create(ZigoDB);
        self.rw_a = try Region.init(allocator, REGION_SIZE);
        self.rw_b = try Region.init(allocator, REGION_SIZE);
        self.region_bit = 0;
        self.total_entries = 0;
        self.total_drains = 0;
        self.last_dump_time = 0;
        self.max_slots = MAX_ENTRIES_PER_REGION;
        return self;
    }

    pub fn getActiveRegion(self: *const ZigoDB) *Region {
        const bit = @atomicLoad(u32, &self.region_bit, .acquire);
        return if (bit == 0) self.rw_a else self.rw_b;
    }

    pub fn getInactiveRegion(self: *const ZigoDB) *Region {
        const bit = @atomicLoad(u32, &self.region_bit, .acquire);
        return if (bit == 0) self.rw_b else self.rw_a;
    }

    pub fn deinit(self: *ZigoDB, allocator: std.mem.Allocator) void {
        self.rw_a.deinit(allocator);
        self.rw_b.deinit(allocator);
        allocator.destroy(self);
    }
};

// ============================================================
// CGO-compatible structures
// ============================================================
pub const SlotReservation = extern struct {
    slot_index: u32,
    region: *Region,
    success: bool,
};

pub const DrainResult = extern struct {
    old_region: *Region,
    entries_written: u32,
    success: bool,
};

// ============================================================
// WriteLock - Ticket mechanism for deterministic ordering
// ============================================================
pub const WriteLock = struct {
    ticket: u32,
    serving: u32,

    pub fn init() WriteLock {
        return WriteLock{ .ticket = 0, .serving = 0 };
    }

    pub fn acquireTicket(self: *WriteLock) u32 {
        return @atomicRmw(u32, &self.ticket, .Add, 1, .acq_rel);
    }

    pub fn waitForTurn(self: *WriteLock, my_ticket: u32) void {
        while (@atomicLoad(u32, &self.serving, .acquire) != my_ticket) {
            std.atomic.spinLoopHint();
        }
    }

    pub fn release(self: *WriteLock) void {
        @atomicRmw(u32, &self.serving, .Add, 1, .release);
    }
};

// ============================================================
// DrainTrigger - Enum for drain conditions
// ============================================================
pub const DrainTrigger = enum {
    SizeThreshold,
    CountThreshold,
    TimeThreshold,
    Forced,
};

// ============================================================
// IndexEntry - For index serialization
// ============================================================
pub const IndexEntry = struct {
    room_id: u32,
    offset: u64,
    uuid: u64,
    timestamp: i64,
    prev_hash: u64,
    state_hash: u64,
};

// ============================================================
// ChunkMetadata & MasterIndex - For chunk tracking
// ============================================================
pub const ChunkMetadata = struct {
    chunk_id: u32,
    filename: [64]u8,
    start_time: i64,
    end_time: i64,
    entry_count: u32,
};

pub const MasterIndex = struct {
    chunks: []ChunkMetadata,
    chunk_count: usize,

    pub fn init(allocator: std.mem.Allocator, max: usize) !*MasterIndex {
        const self = try allocator.create(MasterIndex);
        self.chunks = try allocator.alloc(ChunkMetadata, max);
        self.chunk_count = 0;
        return self;
    }

    pub fn addChunk(self: *MasterIndex, m: ChunkMetadata) void {
        if (self.chunk_count < self.chunks.len) {
            self.chunks[self.chunk_count] = m;
            self.chunk_count += 1;
        }
    }

    pub fn findByTime(self: *const MasterIndex, ts: i64) ?*const ChunkMetadata {
        for (self.chunks[0..self.chunk_count]) |*c| {
            if (ts >= c.start_time and ts <= c.end_time) return c;
        }
        return null;
    }
};

// ============================================================
// Search Pool structures
// ============================================================
pub const Bitmap = [16]u64; // 16 * 64 = 1024 bits

pub const SearchSegment = struct {
    data: [SEGMENT_SIZE]u8,
    chunk_id: u32,
    in_use: bool,
};

pub const SearchPool = struct {
    segments: [MAX_SEGMENTS]SearchSegment,
    bitmap: Bitmap,
    spin_lock: u32,

    pub fn init() SearchPool {
        var p: SearchPool = undefined;
        for (&p.bitmap) |*b| b.* = 0;
        p.spin_lock = 0;
        for (0..MAX_SEGMENTS) |i| {
            p.segments[i] = SearchSegment{ .data = undefined, .chunk_id = 0, .in_use = false };
        }
        return p;
    }
};

// ============================================================
// Temporal Layer structures
// ============================================================
pub const TimeCheckpoint = struct {
    timestamp: i64,
    chunk_id: u32,
    offset: u32,
    state_hash: u64,
};

pub const TemporalIndex = struct {
    checkpoints: []TimeCheckpoint,
    checkpoint_count: usize,

    pub fn init(allocator: std.mem.Allocator, max: usize) !*TemporalIndex {
        const self = try allocator.create(TemporalIndex);
        self.checkpoints = try allocator.alloc(TimeCheckpoint, max);
        self.checkpoint_count = 0;
        return self;
    }

    pub fn addCheckpoint(self: *TemporalIndex, cp: TimeCheckpoint) void {
        if (self.checkpoint_count < self.checkpoints.len) {
            self.checkpoints[self.checkpoint_count] = cp;
            self.checkpoint_count += 1;
        }
    }

    pub fn findNearest(self: *const TemporalIndex, target: i64) ?*const TimeCheckpoint {
        if (self.checkpoint_count == 0) return null;
        var nearest: ?*const TimeCheckpoint = null;
        var min_diff: i64 = std.math.maxInt(i64);
        for (self.checkpoints[0..self.checkpoint_count]) |*c| {
            const diff = @abs(target - c.timestamp);
            if (diff < min_diff) {
                min_diff = diff;
                nearest = c;
            }
        }
        return nearest;
    }
};

pub const CheckpointTrigger = struct {
    entries_since_last: u32,
    time_since_last: i64,

    pub fn init() CheckpointTrigger {
        return .{ .entries_since_last = 0, .time_since_last = 0 };
    }
    pub fn shouldCreate(self: *CheckpointTrigger, count: u32, elapsed: i64) bool {
        self.entries_since_last += count;
        self.time_since_last += elapsed;
        return self.entries_since_last >= 10000 or self.time_since_last >= 30 * 1000 * 1000 * 1000;
    }
    pub fn reset(self: *CheckpointTrigger) void {
        self.* = .{ .entries_since_last = 0, .time_since_last = 0 };
    }
};

// ============================================================
// Replication structures
// ============================================================
pub const GossipMessage = struct {
    chunk_id: u32,
    latest_hash: u64,
    latest_timestamp: i64,
    node_id: u16,
};

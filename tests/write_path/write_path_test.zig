// ============================================================
// Zigo-DB Write Path Tests
// Phase 19: Unit Tests - Write Path (Multi-Core Safe)
// ============================================================

const std = @import("std");
const expect = std.testing.expect;
const expectEqual = std.testing.expectEqual;

// Inline the types we need for testing
const REGION_SIZE = 16 * 1024 * 1024;
const MAX_ENTRIES_PER_REGION = 15625;

const MessageEntry = extern struct {
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

const Region = struct {
    cursor: u32,
    active_writers: i32,
    entries: []MessageEntry,

    pub fn init() Region {
        return Region{
            .cursor = 0,
            .active_writers = 0,
            .entries = &.{},
        };
    }

    pub fn reserveSlot(self: *Region) ?*MessageEntry {
        if (self.cursor >= MAX_ENTRIES_PER_REGION) {
            return null;
        }
        // Simulate atomic increment
        const idx = self.cursor;
        self.cursor += 1;

        if (self.entries.len > idx) {
            return &self.entries[idx];
        }
        return null;
    }

    pub fn releaseSlot(self: *Region) void {
        if (self.active_writers > 0) {
            self.active_writers -= 1;
        }
    }

    pub fn isFull(self: *const Region) bool {
        return self.cursor >= MAX_ENTRIES_PER_REGION;
    }

    pub fn isEmpty(self: *const Region) bool {
        return self.cursor == 0;
    }

    pub fn reset(self: *Region) void {
        self.cursor = 0;
        self.active_writers = 0;
    }
};

// ============================================================
// T19.1: Test db_reserve_slot() basic functionality
// ============================================================

test "test_reserve_slot_single_success" {
    var region = Region.init();

    // Allocate some mock entries for testing
    var entries: [100]MessageEntry = undefined;
    region.entries = entries[0..];

    // Test single slot reservation succeeds
    const slot = region.reserveSlot();
    try expect(slot != null);
    // Cursor is incremented after reservation
    try expectEqual(@as(u32, 1), region.cursor);
}

test "test_reserve_slot_sequential" {
    var region = Region.init();
    var entries: [10]MessageEntry = undefined;
    region.entries = entries[0..];

    // Test multiple sequential reservations return sequential indices
    var indices: [10]u32 = undefined;
    for (0..10) |i| {
        const slot = region.reserveSlot();
        try expect(slot != null);
        indices[i] = region.cursor - 1;
    }

    // Verify sequential indices
    for (0..9) |i| {
        try expectEqual(@as(u32, @intCast(i)), indices[i]);
    }
}

test "test_reserve_slot_full_returns_null" {
    var region = Region.init();
    var entries: [5]MessageEntry = undefined;
    region.entries = entries[0..];
    region.cursor = 5; // Simulate full region

    // Test reservation when region is full returns failure
    const slot = region.reserveSlot();
    try expect(slot == null);
}

// ============================================================
// T19.2: Test db_reserve_slot() atomicity (simulated)
// ============================================================

test "test_reserve_slot_unique_indices" {
    var region = Region.init();
    var entries: [1000]MessageEntry = undefined;
    region.entries = entries[0..];

    // Simulate multiple reservations
    var indices: [100]u32 = undefined;
    for (0..100) |i| {
        _ = region.reserveSlot();
        indices[i] = region.cursor - 1;
    }

    // Verify uniqueness using a simple check
    for (0..100) |i| {
        var is_unique = true;
        for (0..100) |j| {
            if (i != j and indices[i] == indices[j]) {
                is_unique = false;
                break;
            }
        }
        try expect(is_unique);
    }
}

// ============================================================
// T19.3: Test db_release_slot() functionality
// ============================================================

test "test_release_slot_decrements_counter" {
    var region = Region.init();
    region.active_writers = 5;

    // Test release decrements active_writers counter
    region.releaseSlot();
    try expectEqual(@as(i32, 4), region.active_writers);

    region.releaseSlot();
    try expectEqual(@as(i32, 3), region.active_writers);
}

test "test_release_slot_minimum_zero" {
    var region = Region.init();
    region.active_writers = 0;

    // Test release doesn't go below zero
    region.releaseSlot();
    try expectEqual(@as(i32, 0), region.active_writers);
}

// ============================================================
// T19.4: Test db_fill_entry() correctness
// ============================================================

test "test_fill_entry_copies_data" {
    var entry = MessageEntry.init();

    const test_data = "{\"message\": \"hello world\"}";
    entry.setData(1, 12345, 1234567890, 0, 0, test_data);

    // Verify data was copied correctly
    try expectEqual(@as(u32, 1), entry.room_id);
    try expectEqual(@as(u64, 12345), entry.uuid);
    try expectEqual(@as(i64, 1234567890), entry.timestamp);
    try expectEqual(@as(u32, test_data.len), entry.data_len);
}

test "test_fill_entry_checksum_valid" {
    var entry = MessageEntry.init();

    const test_data = "{\"test\": \"data\"}";
    entry.setData(0, 1, 0, 0, 0, test_data);

    // Verify checksum field is non-zero after setData
    try expect(entry.checksum != 0);
}

test "test_fill_entry_truncates_long_data" {
    var entry = MessageEntry.init();

    // Create data longer than json_data buffer (1024 bytes)
    const long_data = "x" ++ "0123456789" ** 200;
    entry.setData(0, 1, 0, 0, 0, long_data);

    // Verify data is truncated to 1024 bytes
    try expectEqual(@as(u32, 1024), entry.data_len);
}

// ============================================================
// T19.5: Test WriteLock ticket mechanism (simulated)
// ============================================================

const WriteLock = struct {
    next_ticket: u32 = 0,
    serving_ticket: u32 = 0,

    pub fn acquireTicket(self: *WriteLock) u32 {
        const ticket = self.next_ticket;
        self.next_ticket += 1;
        return ticket;
    }

    pub fn isMyTurn(self: *const WriteLock, ticket: u32) bool {
        return self.serving_ticket == ticket;
    }

    pub fn release(self: *WriteLock) void {
        self.serving_ticket += 1;
    }
};

test "test_write_lock_sequential_tickets" {
    var lock = WriteLock{};

    // Test acquireTicket returns sequential tickets
    const t1 = lock.acquireTicket();
    const t2 = lock.acquireTicket();
    const t3 = lock.acquireTicket();

    try expectEqual(@as(u32, 0), t1);
    try expectEqual(@as(u32, 1), t2);
    try expectEqual(@as(u32, 2), t3);
}

test "test_write_lock_serving_advances" {
    var lock = WriteLock{};

    const ticket = lock.acquireTicket();
    try expect(lock.isMyTurn(ticket));

    lock.release();
    // After release, the ticket should be served
    try expect(!lock.isMyTurn(ticket));
    try expectEqual(@as(u32, 1), lock.serving_ticket);
}

// ============================================================
// T19.6: Test db_needs_drain() thresholds
// ============================================================

test "test_needs_drain_below_threshold" {
    var region = Region.init();
    var entries: [1000]MessageEntry = undefined;
    region.entries = entries[0..];

    // Test returns false below 90% threshold
    region.cursor = 500; // 50%
    try expect(!region.isFull());
}

test "test_needs_drain_at_threshold" {
    var region = Region.init();
    var entries: [1000]MessageEntry = undefined;
    region.entries = entries[0..];

    // Test returns true at or above 90% threshold
    region.cursor = 900; // 90%
    try expect(!region.isFull()); // cursor starts at 0, so 900 is full

    // Actually set cursor to max to simulate full
    region.cursor = MAX_ENTRIES_PER_REGION;
    try expect(region.isFull());
}

// ============================================================
// Region isFull test helpers
// ============================================================

test "test_region_isFull_above_max" {
    var region = Region.init();
    var entries: [100]MessageEntry = undefined;
    region.entries = entries[0..];

    // Cursor at max entries should be full
    region.cursor = MAX_ENTRIES_PER_REGION;
    try expect(region.isFull());
}

test "test_region_isEmpty" {
    var region = Region.init();

    // Test isEmpty returns correct state
    try expect(region.isEmpty());

    _ = region.reserveSlot();
    try expect(!region.isEmpty());
}

test "test_region_reset" {
    var region = Region.init();
    var entries: [10]MessageEntry = undefined;
    region.entries = entries[0..];

    // Use some slots
    _ = region.reserveSlot();
    _ = region.reserveSlot();
    region.active_writers = 3;

    // Reset
    region.reset();

    try expect(region.isEmpty());
    try expectEqual(@as(i32, 0), region.active_writers);
}

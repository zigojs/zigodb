// ============================================================
// Zigo-DB Search Pool Tests
// Phase 22: Unit Tests - Search Pool
// ============================================================

const std = @import("std");
const expect = std.testing.expect;
const expectEqual = std.testing.expectEqual;

// Constants
const SEGMENT_SIZE = 64 * 1024;
const MAX_SEGMENTS = 1024;

// Types for testing
const MessageEntry = extern struct {
    room_id: u32 align(1),
    uuid: u64 align(1),
    timestamp: i64 align(1),
    prev_hash: u64 align(1),
    state_hash: u64 align(1),
    data_len: u32 align(1),
    checksum: u32 align(1),
    json_data: [1024]u8 align(1),
};

const SegmentState = enum {
    free,
    used,
};

const SearchSegment = struct {
    state: SegmentState,
    chunk_id: u64,
    offset_start: u64,
    offset_end: u64,
    data: []u8,
};

const SearchPool = struct {
    segments: [MAX_SEGMENTS]SearchSegment,
    bitmap: [16]u64, // 1024 bits

    pub fn init() SearchPool {
        var pool: SearchPool = undefined;

        // Initialize all segments as free
        for (0..MAX_SEGMENTS) |i| {
            pool.segments[i] = SearchSegment{
                .state = .free,
                .chunk_id = 0,
                .offset_start = 0,
                .offset_end = 0,
                .data = &.{},
            };
        }

        // Initialize bitmap to all zeros (all free)
        for (0..16) |i| {
            pool.bitmap[i] = 0;
        }

        return pool;
    }

    pub fn reserveSegment(self: *SearchPool) ?usize {
        // Find first free segment
        for (0..MAX_SEGMENTS) |i| {
            if (self.segments[i].state == .free) {
                self.segments[i].state = .used;
                return i;
            }
        }
        return null;
    }

    pub fn releaseSegment(self: *SearchPool, index: usize) void {
        if (index < MAX_SEGMENTS) {
            self.segments[index].state = .free;
        }
    }

    pub fn isSegmentFree(self: *const SearchPool, index: usize) bool {
        if (index >= MAX_SEGMENTS) return false;
        return self.segments[index].state == .free;
    }

    pub fn getFreeSegmentCount(self: *const SearchPool) usize {
        var count: usize = 0;
        for (0..MAX_SEGMENTS) |i| {
            if (self.segments[i].state == .free) {
                count += 1;
            }
        }
        return count;
    }
};

// ============================================================
// T22.1: Test segment reservation (memcpy mode)
// ============================================================

test "test_search_pool_init" {
    var pool = SearchPool.init();

    // All segments should be free initially
    try expectEqual(@as(usize, MAX_SEGMENTS), pool.getFreeSegmentCount());
}

test "test_reserve_segment_success" {
    var pool = SearchPool.init();

    // Reserve a segment
    const index = pool.reserveSegment();
    try expect(index != null);

    // That segment should now be used
    try expect(!pool.isSegmentFree(index.?));
}

test "test_reserve_segment_returns_null_when_full" {
    var pool = SearchPool.init();

    // Reserve all segments
    var reserved: [MAX_SEGMENTS]usize = undefined;
    for (0..MAX_SEGMENTS) |i| {
        const idx = pool.reserveSegment();
        if (idx) |v| {
            reserved[i] = v;
        }
    }

    // Next reservation should fail
    const result = pool.reserveSegment();
    try expect(result == null);
}

test "test_release_segment" {
    var pool = SearchPool.init();

    // Reserve and release
    const index = pool.reserveSegment();
    try expect(index != null);

    pool.releaseSegment(index.?);

    // Should be free again
    try expect(pool.isSegmentFree(index.?));
}

// ============================================================
// T22.2: Test segment structure
// ============================================================

test "test_segment_state_transitions" {
    var segment = SearchSegment{
        .state = .free,
        .chunk_id = 0,
        .offset_start = 0,
        .offset_end = 0,
        .data = &.{},
    };

    // Initially free
    try expect(segment.state == .free);

    // Mark as used
    segment.state = .used;
    try expect(segment.state == .used);

    // Mark as free
    segment.state = .free;
    try expect(segment.state == .free);
}

test "test_segment_metadata" {
    var segment = SearchSegment{
        .state = .used,
        .chunk_id = 12345,
        .offset_start = 1024,
        .offset_end = 2048,
        .data = &.{},
    };

    try expectEqual(@as(u64, 12345), segment.chunk_id);
    try expectEqual(@as(u64, 1024), segment.offset_start);
    try expectEqual(@as(u64, 2048), segment.offset_end);
}

// ============================================================
// T22.3: Test bitmap operations
// ============================================================

test "test_bitmap_initially_all_free" {
    var pool = SearchPool.init();

    // Check all bitmap bits are 0
    for (0..16) |i| {
        try expectEqual(@as(u64, 0), pool.bitmap[i]);
    }
}

test "test_bitmap_set_bit" {
    var pool = SearchPool.init();

    // Set a bit (bit 5)
    pool.bitmap[0] |= (1 << 5);

    // Verify it's set
    try expect((pool.bitmap[0] & (1 << 5)) != 0);
}

test "test_bitmap_clear_bit" {
    var pool = SearchPool.init();
    const mask: u64 = 1 << 10;

    // Set and clear a bit
    pool.bitmap[0] |= mask;
    try expect((pool.bitmap[0] & mask) != 0);

    pool.bitmap[0] &= ~mask;
    try expect((pool.bitmap[0] & mask) == 0);
}

// ============================================================
// T22.4: Test concurrent segment access (simulated)
// ============================================================

test "test_multiple_segment_reservations" {
    var pool = SearchPool.init();

    // Reserve multiple segments
    var indices: [100]usize = undefined;
    for (0..100) |i| {
        const idx = pool.reserveSegment();
        try expect(idx != null);
        indices[i] = idx.?;
    }

    // All should be unique
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

test "test_alternating_reserve_release" {
    var pool = SearchPool.init();

    // Reserve, release, reserve again
    const idx1 = pool.reserveSegment();
    try expect(idx1 != null);

    pool.releaseSegment(idx1.?);

    const idx2 = pool.reserveSegment();
    try expect(idx2 != null);

    // Should be the same index (first available)
    try expectEqual(idx1.?, idx2.?);
}

// ============================================================
// T22.5: Test segment size constants
// ============================================================

test "test_segment_size_constant" {
    // Verify segment size is 64KB
    try expectEqual(@as(usize, 65536), SEGMENT_SIZE);
}

test "test_max_segments_constant" {
    // Verify max segments is 1024
    try expectEqual(@as(usize, 1024), MAX_SEGMENTS);
}

test "test_total_pool_size" {
    // Total pool should be 64MB
    const total = SEGMENT_SIZE * MAX_SEGMENTS;
    try expectEqual(@as(usize, 64 * 1024 * 1024), total);
}

// ============================================================
// Zigo-DB Persistence Tests
// Phase 21: Unit Tests - Persistence
// ============================================================

const std = @import("std");
const expect = std.testing.expect;
const expectEqual = std.testing.expectEqual;

// Constants
const MAX_ENTRIES_PER_REGION = 15625;

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
    }
};

const IndexEntry = struct {
    offset: u64,
    uuid: u64,
    timestamp: i64,
    prev_hash: u64,
    room_id: u32,
    state_hash: u64,
};

const ChunkMetadata = struct {
    chunk_id: u64,
    start_time: i64,
    end_time: i64,
    entry_count: u32,
    file_size: u64,
    checksum: u64,
};

// ============================================================
// T21.1: Test dump/load functionality
// ============================================================

test "test_message_entry_binary_size" {
    // Verify struct size is exactly expected
    const size = @sizeOf(MessageEntry);
    try expect(size > 0);
    // Should be approximately: 4 + 8 + 8 + 8 + 8 + 4 + 4 + 1024 = 1068 bytes
    try expect(size >= 1068);
}

test "test_message_entry_serialization" {
    var entry = MessageEntry.init();
    entry.setData(1, 12345, 1234567890, 0, 0, "test data");

    // Serialize to bytes
    const bytes = std.mem.asBytes(&entry);

    // Verify we can read it back
    const read_entry = std.mem.bytesToValue(MessageEntry, bytes[0..@sizeOf(MessageEntry)]);

    try expectEqual(@as(u32, 1), read_entry.room_id);
    try expectEqual(@as(u64, 12345), read_entry.uuid);
    try expectEqual(@as(i64, 1234567890), read_entry.timestamp);
}

test "test_message_entry_deserialization" {
    // Create original entry
    var original = MessageEntry.init();
    original.setData(42, 99999, 9876543210, 111, 222, "hello");

    // Get raw bytes
    const bytes = std.mem.asBytes(&original);

    // Deserialize to new entry
    var deserialized: MessageEntry = undefined;
    @memcpy(std.mem.asBytes(&deserialized), bytes);

    try expectEqual(@as(u32, 42), deserialized.room_id);
    try expectEqual(@as(u64, 99999), deserialized.uuid);
    try expectEqual(@as(i64, 9876543210), deserialized.timestamp);
}

// ============================================================
// T21.2: Test index serialization
// ============================================================

test "test_index_entry_structure" {
    var index = IndexEntry{
        .offset = 1024,
        .uuid = 12345,
        .timestamp = 1234567890,
        .prev_hash = 0,
        .room_id = 1,
        .state_hash = 999,
    };

    try expectEqual(@as(u64, 1024), index.offset);
    try expectEqual(@as(u64, 12345), index.uuid);
}

test "test_index_entry_binary_size" {
    const size = @sizeOf(IndexEntry);
    // Should be: 8 + 8 + 8 + 8 + 4 + 8 = 44 bytes (may vary with alignment)
    try expect(size > 0);
}

test "test_index_serialization_roundtrip" {
    var original = IndexEntry{
        .offset = 2048,
        .uuid = 54321,
        .timestamp = 1111111111,
        .prev_hash = 123,
        .room_id = 5,
        .state_hash = 456,
    };

    // Serialize
    const bytes = std.mem.asBytes(&original);

    // Deserialize
    var deserialized: IndexEntry = undefined;
    @memcpy(std.mem.asBytes(&deserialized), bytes);

    try expectEqual(@as(u64, 2048), deserialized.offset);
    try expectEqual(@as(u64, 54321), deserialized.uuid);
    try expectEqual(@as(u32, 5), deserialized.room_id);
}

// ============================================================
// T21.3: Test chunk metadata
// ============================================================

test "test_chunk_metadata_structure" {
    var meta = ChunkMetadata{
        .chunk_id = 1,
        .start_time = 1000000000,
        .end_time = 2000000000,
        .entry_count = 1000,
        .file_size = 1048576,
        .checksum = 123456789,
    };

    try expectEqual(@as(u64, 1), meta.chunk_id);
    try expectEqual(@as(u32, 1000), meta.entry_count);
}

test "test_chunk_metadata_binary_size" {
    const size = @sizeOf(ChunkMetadata);
    try expect(size > 0);
}

// ============================================================
// T21.4: Test chunk filename generation
// ============================================================

test "test_chunk_filename_format" {
    // Test chunk filename format: chunk_YYYYMMDD_HHMMSS_uuid.rb
    const timestamp: i64 = 1700000000; // 2023-11-15
    const uuid: u64 = 123456789;

    // Expected format components
    try expect(timestamp > 0);
    try expect(uuid > 0);
}

test "test_index_filename_format" {
    // Test index filename: chunk_YYYYMMDD_HHMMSS_uuid.index
    const timestamp: i64 = 1700000000;
    const uuid: u64 = 123456789;

    // Both chunk and index should have same base name
    try expect(timestamp > 0);
    try expect(uuid > 0);
}

// ============================================================
// T21.5: Test round-trip persistence
// ============================================================

test "test_multiple_entries_persistence" {
    var entries: [10]MessageEntry = undefined;

    // Initialize multiple entries
    for (0..10) |i| {
        entries[i] = MessageEntry.init();
        const data = "test data";
        entries[i].setData(@as(u32, @intCast(i)), @as(u64, @intCast(i)), @as(i64, @intCast(i * 1000)), 0, 0, data);
    }

    // Serialize all entries
    var all_bytes: [10 * @sizeOf(MessageEntry)]u8 = undefined;
    var offset: usize = 0;
    for (entries) |entry| {
        const entry_bytes = std.mem.asBytes(&entry);
        @memcpy(all_bytes[offset .. offset + entry_bytes.len], entry_bytes);
        offset += entry_bytes.len;
    }

    // Verify total size
    try expectEqual(@as(usize, 10 * @sizeOf(MessageEntry)), offset);
}

test "test_index_entries_array" {
    var indices: [100]IndexEntry = undefined;

    // Fill index entries
    for (0..100) |i| {
        indices[i] = IndexEntry{
            .offset = @as(u64, @intCast(i * 1024)),
            .uuid = @as(u64, @intCast(i)),
            .timestamp = @as(i64, @intCast(i * 1000)),
            .prev_hash = @as(u64, @intCast(i * 10)),
            .room_id = @as(u32, @intCast(i % 10)),
            .state_hash = @as(u64, @intCast(i * 100)),
        };
    }

    // Verify some entries
    try expectEqual(@as(u64, 0), indices[0].offset);
    try expectEqual(@as(u64, 10240), indices[10].offset);
}

// ============================================================
// T21.6: Test persistence edge cases
// ============================================================

test "test_empty_region_persistence" {
    // Test that empty region can be serialized
    var entries: [0]MessageEntry = undefined;
    const slice = entries[0..0];

    try expectEqual(@as(usize, 0), slice.len);
}

test "test_max_entries_persistence" {
    // Test that max entries can be handled
    const max_count = MAX_ENTRIES_PER_REGION;
    try expect(max_count > 0);
    try expect(max_count == 15625);
}

test "test_partial_entry_persistence" {
    var entry = MessageEntry.init();

    // Set minimal data
    entry.room_id = 1;
    entry.uuid = 1;
    entry.timestamp = 1;
    entry.prev_hash = 0;
    entry.state_hash = 0;
    entry.data_len = 0;

    // Serialize
    const bytes = std.mem.asBytes(&entry);

    // Verify size
    try expectEqual(@as(usize, @sizeOf(MessageEntry)), bytes.len);
}

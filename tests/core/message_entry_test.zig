// ============================================================
// T18.1 - MessageEntry Unit Tests
// ============================================================

const std = @import("std");
const testing = std.testing;

// Define MessageEntry locally for testing (matches db/types.zig)
const MessageEntry = extern struct {
    room_id: u32 align(1),
    uuid: u64 align(1),
    timestamp: i64 align(1),
    prev_hash: u64 align(1),
    state_hash: u64 align(1),
    data_len: u32 align(1),
    checksum: u32 align(1),
    json_data: [1024]u8 align(1),

    fn init() MessageEntry {
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

    fn setData(self: *MessageEntry, room_id: u32, uuid: u64, timestamp: i64, prev_hash: u64, state_hash: u64, data: []const u8) void {
        self.room_id = room_id;
        self.uuid = uuid;
        self.timestamp = timestamp;
        self.prev_hash = prev_hash;
        self.state_hash = state_hash;
        self.data_len = @as(u32, @intCast(@min(data.len, self.json_data.len)));
        @memcpy(self.json_data[0..self.data_len], data[0..self.data_len]);
        self.checksum = self.calculateChecksum();
    }

    fn calculateChecksum(self: *MessageEntry) u32 {
        var hash: u32 = 0;
        const bytes = std.mem.asBytes(self);
        for (bytes) |b| {
            hash = hash *% 31 + b;
        }
        return hash;
    }
};

test "MessageEntry init creates zeroed entry" {
    const entry = MessageEntry.init();

    try testing.expectEqual(@as(u32, 0), entry.room_id);
    try testing.expectEqual(@as(u64, 0), entry.uuid);
    try testing.expectEqual(@as(i64, 0), entry.timestamp);
    try testing.expectEqual(@as(u64, 0), entry.prev_hash);
    try testing.expectEqual(@as(u64, 0), entry.state_hash);
    try testing.expectEqual(@as(u32, 0), entry.data_len);
    try testing.expectEqual(@as(u32, 0), entry.checksum);
}

test "MessageEntry setData fills all fields correctly" {
    var entry = MessageEntry.init();

    const test_data = "{\"message\": \"hello world\"}";
    entry.setData(
        1, // room_id
        12345, // uuid
        1699999999000000000, // timestamp (ns)
        0xDEADBEEF, // prev_hash
        0xCAFEBABE, // state_hash
        test_data,
    );

    try testing.expectEqual(@as(u32, 1), entry.room_id);
    try testing.expectEqual(@as(u64, 12345), entry.uuid);
    try testing.expectEqual(@as(i64, 1699999999000000000), entry.timestamp);
    try testing.expectEqual(@as(u64, 0xDEADBEEF), entry.prev_hash);
    try testing.expectEqual(@as(u64, 0xCAFEBABE), entry.state_hash);
    try testing.expectEqual(@as(u32, test_data.len), entry.data_len);

    // Verify data was copied
    try testing.expectEqualSlices(u8, test_data, entry.json_data[0..entry.data_len]);
}

test "MessageEntry calculateChecksum is consistent" {
    var entry = MessageEntry.init();

    const test_data = "test message";
    entry.setData(1, 100, 1000, 0, 0, test_data);

    const checksum1 = entry.calculateChecksum();
    const checksum2 = entry.calculateChecksum();

    try testing.expectEqual(checksum1, checksum2);
}

test "MessageEntry setData generates non-zero checksum" {
    var entry = MessageEntry.init();

    const test_data = "some data";
    entry.setData(1, 100, 1000, 0, 0, test_data);

    // After setData, checksum should be non-zero
    try testing.expect(entry.checksum != 0);
}

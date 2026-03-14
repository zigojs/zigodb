#ifndef ZIGO_DB_H
#define ZIGO_DB_H

#include <stdint.h>
#include <stdbool.h>

// Forward declaration
typedef struct ZigoDB ZigoDB;

// Slot reservation result
typedef struct {
    uint32_t slot_index;
    void* region;
    bool success;
} SlotReservation;

// Drain result
typedef struct {
    void* old_region;
    uint32_t entries_written;
    bool success;
} DrainResult;

// Database initialization
extern ZigoDB* db_init(void);
extern void db_shutdown(void);

// Slot operations
extern SlotReservation db_reserve_slot_cgo(void);
extern void db_release_slot_cgo(void);
extern void db_fill_entry_cgo(
    uint32_t slot_index,
    uint32_t room_id,
    uint64_t uuid,
    int64_t timestamp,
    uint64_t prev_hash,
    uint64_t state_hash,
    const uint8_t* data,
    size_t data_len
);

// Fast write path (Locker Writer)
extern int32_t db_write_raw_cgo(uint32_t room_id, const uint8_t* data, uint32_t data_len);

// Drain operations
extern DrainResult db_switch_and_drain_cgo(void);
extern bool db_needs_drain_cgo(void);
extern bool db_validate_drain(void* region);

// Status
extern uint64_t db_get_status_cgo(void);
extern uint32_t db_get_cursor_cgo(void);
extern void* db_get_last_entry_cgo(void);
extern void* db_get_entry_cgo(uint32_t index);

// Dump operations
extern bool db_dump_cgo(const uint8_t* filepath, size_t filepath_len);
extern bool db_dump_index_cgo(const uint8_t* filepath, size_t filepath_len);

// Search pool operations
extern uint32_t search_reserve_segment_cgo(uint32_t chunk_id, const uint8_t* data, size_t data_len);
extern void search_release_segment_cgo(uint32_t segment_id);
extern size_t search_query_cgo(
    uint32_t chunk_id,
    const uint8_t* query,
    size_t query_len,
    void* results,
    size_t max_results
);

#endif // ZIGO_DB_H

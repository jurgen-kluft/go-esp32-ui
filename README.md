


# Asset Store

An Asset has an index and an offset into a memory arena.

```c++
struct asset_t
{
    u32 m_type : 8;
    u32 m_size : 24;
    u32 m_offset;
};

#define ASSET_MAX_COUNT 8192

asset_t g_asset_store[ASSET_MAX_COUNT];
byte*   g_asset_store_arena;             // PSRAM pointer to the asset store arena
u32     g_asset_store_arena_pos;         // Current position in the asset store arena
u32     g_asset_store_arena_size;        // Size of the asset store arena in bytes

void g_add_asset(u16 index, u8 type, u32 size, byte* data)
{
    g_asset_store[index].m_type = type;
    g_asset_store[index].m_size = size;
    g_asset_store[index].m_offset = g_asset_store_arena_pos;
    memcpy(g_asset_store_arena + g_asset_store_arena_pos, data, size);
    g_asset_store_arena_pos += size;
}

struct font_t
{

};

struct palette_t
{

};

struct image_t
{

};



```
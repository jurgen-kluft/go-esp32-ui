#include "vq.h"
#include <string.h>  // Required for raw fast block memcpy operations

bool vq_get_metadata(const uint8_t* file_bytes, uint16_t* out_width, uint16_t* out_height, uint16_t* out_mode)
{
    if (!file_bytes)
        return false;

    // Direct mapping from the 10-byte unpadded structure
    memcpy(out_mode, &file_bytes[0], 2);
    memcpy(out_width, &file_bytes[2], 2);
    memcpy(out_height, &file_bytes[4], 2);

    return true;
}

bool vq_decompress_frame(const uint8_t* file_bytes, uint16_t* fb_psram_dest, uint16_t* sram_strip_buf)
{
    if (!file_bytes || !fb_psram_dest || !sram_strip_buf)
    {
        return false;
    }

    // 1. Process 10-Byte Container Header Descriptor
    uint16_t mode        = 0;
    uint16_t width       = 0;
    uint16_t height      = 0;
    uint16_t cb2x2_count = 0;
    uint16_t cb4x4_count = 0;

    memcpy(&mode, &file_bytes[0], 2);
    memcpy(&width, &file_bytes[2], 2);
    memcpy(&height, &file_bytes[4], 2);
    memcpy(&cb2x2_count, &file_bytes[6], 2);
    memcpy(&cb4x4_count, &file_bytes[8], 2);

    uint32_t grid_x       = width / 4;
    uint32_t grid_y       = height / 4;
    uint32_t total_blocks = grid_x * grid_y;

    // Set read pointer directly past the 10-byte header block
    uint32_t src_ptr = 10;

    // Strategy 0 Fallback Handler: If image is stored as an uncompressed raw sequence
    if (mode == 0)
    {
        memcpy(fb_psram_dest, &file_bytes[src_ptr], width * height * sizeof(uint16_t));
        return true;
    }

    // 2. Map Zero-Copy Pointers sequentially from the file payload
    const uint16_t* map_grid = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);
    src_ptr += total_blocks * sizeof(uint16_t);

    const uint16_t* cb2x2_flat = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);
    src_ptr += cb2x2_count * 4 * sizeof(uint16_t);  // 4 pixels (uint16_t) per 2x2 cluster = 8 bytes

    // The 4x4 Codebook sits sequentially directly after the 2x2 dictionary
    // Each 4x4 block contains 4 indices (uint16_t) pointing to the 2x2 blocks = 8 bytes
    const uint16_t* codebook_4x4 = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);

    // 3. Execution Pipeline Decompress Loop
    uint32_t grid_idx = 0;
    for (uint32_t by = 0; by < grid_y; ++by)
    {
        // Horizontal Line Construction Step: Rebuild 4 rows completely inside internal SRAM
        for (uint32_t bx = 0; bx < grid_x; ++bx)
        {
            uint16_t c4x4_idx = map_grid[grid_idx++];

            // Multiply index by 4 via shifting to look up the 4 sub-quadrants inside the 4x4 codebook
            uint32_t c4x4_offset = c4x4_idx << 2;
            uint16_t q0          = codebook_4x4[c4x4_offset + 0];
            uint16_t q1          = codebook_4x4[c4x4_offset + 1];
            uint16_t q2          = codebook_4x4[c4x4_offset + 2];
            uint16_t q3          = codebook_4x4[c4x4_offset + 3];

            uint32_t sram_x = bx * 4;

            // Multiply the 2x2 cluster indices by 4 via shifting to map onto the flat pixel data array
            uint32_t o0 = q0 << 2;
            uint32_t o1 = q1 << 2;
            uint32_t o2 = q2 << 2;
            uint32_t o3 = q3 << 2;

            // Row 0 of SRAM cache strip
            sram_strip_buf[(0 * width) + sram_x + 0] = cb2x2_flat[o0 + 0];
            sram_strip_buf[(0 * width) + sram_x + 1] = cb2x2_flat[o0 + 1];
            sram_strip_buf[(0 * width) + sram_x + 2] = cb2x2_flat[o1 + 0];
            sram_strip_buf[(0 * width) + sram_x + 3] = cb2x2_flat[o1 + 1];

            // Row 1 of SRAM cache strip
            sram_strip_buf[(1 * width) + sram_x + 0] = cb2x2_flat[o0 + 2];
            sram_strip_buf[(1 * width) + sram_x + 1] = cb2x2_flat[o0 + 3];
            sram_strip_buf[(1 * width) + sram_x + 2] = cb2x2_flat[o1 + 2];
            sram_strip_buf[(1 * width) + sram_x + 3] = cb2x2_flat[o1 + 3];

            // Row 2 of SRAM cache strip
            sram_strip_buf[(2 * width) + sram_x + 0] = cb2x2_flat[o2 + 0];
            sram_strip_buf[(2 * width) + sram_x + 1] = cb2x2_flat[o2 + 1];
            sram_strip_buf[(2 * width) + sram_x + 2] = cb2x2_flat[o3 + 0];
            sram_strip_buf[(2 * width) + sram_x + 3] = cb2x2_flat[o3 + 1];

            // Row 3 of SRAM cache strip
            sram_strip_buf[(3 * width) + sram_x + 0] = cb2x2_flat[o2 + 2];
            sram_strip_buf[(3 * width) + sram_x + 1] = cb2x2_flat[o2 + 3];
            sram_strip_buf[(3 * width) + sram_x + 2] = cb2x2_flat[o3 + 2];
            sram_strip_buf[(3 * width) + sram_x + 3] = cb2x2_flat[o3 + 3];
        }

        // Burst-write Step: Flush the 4 fully reconstructed lines out to the external PSRAM Framebuffer
        uint32_t fb_pixel_offset = by * 4 * width;
        memcpy(&fb_psram_dest[fb_pixel_offset], sram_strip_buf, width * 4 * sizeof(uint16_t));
    }

    return true;
}

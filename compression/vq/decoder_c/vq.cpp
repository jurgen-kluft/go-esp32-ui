#include "vq.h"
#include <string.h> // Required for raw fast block memcpy operations

extern "C" {

bool vq_get_metadata(const uint8_t* file_bytes, uint16_t* out_width, uint16_t* out_height, uint16_t* out_mode) {
    if (!file_bytes) return false;

    // Direct mapping from the 10-byte unpadded structure
    memcpy(out_mode,   &file_bytes[0], 2);
    memcpy(out_width,  &file_bytes[2], 2);
    memcpy(out_height, &file_bytes[4], 2);

    return true;
}

bool vq_decompress_frame_circular(const uint8_t* file_bytes, uint16_t* fb_psram_dest, uint16_t* sram_strip_buf) {
    if (!file_bytes || !fb_psram_dest || !sram_strip_buf) {
        return false;
    }

    // 1. Process Container Header Descriptor
    const uint16_t* hdr = (const uint16_t*)file_bytes;
    const uint16_t mode = *hdr++;
    const uint16_t width = *hdr++;
    const uint16_t height = *hdr++;
    const uint16_t cb2x2_count = *hdr++;
    const uint16_t cb4x4_count = *hdr++;

    const uint32_t grid_x = width / 4;
    const uint32_t grid_y = height / 4;
    const uint32_t total_blocks = grid_x * grid_y;

    // Set read pointer directly past the 10-byte header block
    const uint8_t* src_ptr = (const uint8_t*)hdr;

    // Strategy 0 Fallback Handler: If image is stored as an uncompressed raw sequence
    if (mode == 0) {
        memcpy(fb_psram_dest, src_ptr, width * height * sizeof(uint16_t));
        return true;
    }

    // 2. Map Zero-Copy Pointers sequentially from the file payload
    const uint16_t* map_grid = reinterpret_cast<const uint16_t*>(src_ptr);
    src_ptr += total_blocks * sizeof(uint16_t);

    const uint16_t* cb2x2_flat = reinterpret_cast<const uint16_t*>(src_ptr);
    src_ptr += cb2x2_count * 4 * sizeof(uint16_t); // 4 pixels per circular 2x2 cluster = 8 bytes

    // The 4x4 Codebook sits sequentially directly after the 2x2 dictionary
    const uint16_t* codebook_4x4 = reinterpret_cast<const uint16_t*>(src_ptr);

    // 3. Reconstruct Grid Pipeline Execution
    uint32_t grid_idx = 0;
    for (uint32_t by = 0; by < grid_y; ++by) {
        
        // Build the 4-row horizontal slice completely in internal SRAM scratchpad
        for (uint32_t bx = 0; bx < grid_x; ++bx) {
            uint16_t c4x4_idx = map_grid[grid_idx++];

            // Multiply index by 4 via shifting to look up the 4 sub-quadrants inside the 4x4 codebook
            uint32_t c4x4_offset = c4x4_idx << 2;
            uint16_t raw_q0 = codebook_4x4[c4x4_offset + 0];
            uint16_t raw_q1 = codebook_4x4[c4x4_offset + 1];
            uint16_t raw_q2 = codebook_4x4[c4x4_offset + 2];
            uint16_t raw_q3 = codebook_4x4[c4x4_offset + 3];

            // Isolate upper 2 bits via shift to get the pre-inverted decoder rotation markers
            uint32_t r0 = raw_q0 >> 14; 
            uint32_t r1 = raw_q1 >> 14;
            uint32_t r2 = raw_q2 >> 14; 
            uint32_t r3 = raw_q3 >> 14;

            // Mask out upper bits to fetch pure dictionary memory address indices
            uint32_t o0 = (raw_q0 & 0x3FFF) << 2;
            uint32_t o1 = (raw_q1 & 0x3FFF) << 2;
            uint32_t o2 = (raw_q2 & 0x3FFF) << 2;
            uint32_t o3 = (raw_q3 & 0x3FFF) << 2;

            uint32_t sram_x = bx * 4;

            // Reconstruct Row 0 of SRAM cache strip (Top rows of q0 and q1 quadrants)
            // Clockwise Order Offset: Top-Left Pos = 0, Top-Right Pos = 1
            sram_strip_buf[(0 * width) + sram_x + 0] = cb2x2_flat[o0 + ((r0 + 0) & 3)];
            sram_strip_buf[(0 * width) + sram_x + 1] = cb2x2_flat[o0 + ((r0 + 1) & 3)];
            sram_strip_buf[(0 * width) + sram_x + 2] = cb2x2_flat[o1 + ((r1 + 0) & 3)];
            sram_strip_buf[(0 * width) + sram_x + 3] = cb2x2_flat[o1 + ((r1 + 1) & 3)];

            // Reconstruct Row 1 of SRAM cache strip (Bottom rows of q0 and q1 quadrants)
            // Clockwise Order Offset: Bottom-Left Pos = 3, Bottom-Right Pos = 2
            sram_strip_buf[(1 * width) + sram_x + 0] = cb2x2_flat[o0 + ((r0 + 3) & 3)];
            sram_strip_buf[(1 * width) + sram_x + 1] = cb2x2_flat[o0 + ((r0 + 2) & 3)];
            sram_strip_buf[(1 * width) + sram_x + 2] = cb2x2_flat[o1 + ((r1 + 3) & 3)];
            sram_strip_buf[(1 * width) + sram_x + 3] = cb2x2_flat[o1 + ((r1 + 2) & 3)];

            // Reconstruct Row 2 of SRAM cache strip (Top rows of q2 and q3 quadrants)
            sram_strip_buf[(2 * width) + sram_x + 0] = cb2x2_flat[o2 + ((r2 + 0) & 3)];
            sram_strip_buf[(2 * width) + sram_x + 1] = cb2x2_flat[o2 + ((r2 + 1) & 3)];
            sram_strip_buf[(2 * width) + sram_x + 2] = cb2x2_flat[o3 + ((r3 + 0) & 3)]; 
            sram_strip_buf[(2 * width) + sram_x + 3] = cb2x2_flat[o3 + ((r3 + 1) & 3)];

            // Reconstruct Row 3 of SRAM cache strip (Bottom rows of q2 and q3 quadrants)
            sram_strip_buf[(3 * width) + sram_x + 0] = cb2x2_flat[o2 + ((r2 + 3) & 3)];
            sram_strip_buf[(3 * width) + sram_x + 1] = cb2x2_flat[o2 + ((r2 + 2) & 3)];
            sram_strip_buf[(3 * width) + sram_x + 2] = cb2x2_flat[o3 + ((r3 + 3) & 3)];
            sram_strip_buf[(3 * width) + sram_x + 3] = cb2x2_flat[o3 + ((r3 + 2) & 3)];
        }

        // Flush the 4 fully formed lines out to the external PSRAM Framebuffer layout all at once
        uint32_t fb_pixel_offset = by * 4 * width;
        memcpy(&fb_psram_dest[fb_pixel_offset], sram_strip_buf, width * 4 * sizeof(uint16_t));
    }

    return true;
}

} // extern "C"

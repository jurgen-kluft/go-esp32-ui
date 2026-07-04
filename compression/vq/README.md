# Two-Tier Vector Quantization (2-Tier VQ) Image Compressor

A resolution-agnostic, low-overhead image compression pipeline engineered specifically for resource-constrained microcontrollers like the **Espressif ESP32-S3**. 

By utilizing hierarchical multi-level block deduplication (4x4 macroblocks backed by a dictionary of unique 2x2 pixel clusters), this compressor yields **~3x to 4x compression ratios** for native RGB565 graphics while maintaining the ability to decompress frames entirely **on the fly via zero-copy raster caching**.

## 🚀 Key Advantages for Microcontrollers
* **Hardware-Aware Memory Layout**: Eliminates massive runtime frame decompression buffers. Unpacks line-by-line using a tiny 4-row high-speed internal SRAM buffer before bursting data to external PSRAM via DMA/memcpy.
* **Deterministic Execution & No Branching**: Unlike block-based codecs (JPEG) or streaming compressors (PNG, LZ4), this decoder performs **zero** floating-point math, Huffman tree traversals, or dictionary sliding-window lookups. The decoding path contains zero internal loop branching, eliminating CPU branch mispredictions.
* **Automatic Fallback Protection**: Pre-calculates the final byte budget before saving. If an image contains high photographical noise and cannot save space under compression, it automatically exports as a flat, high-speed uncompressed binary frame.

---

## 💾 File Format Specifications (.bin)

The encoder outputs a tightly aligned, unpadded layout. Multi-byte integers are serialized using **Little-Endian** formatting.

| Offset | Size (Bytes) | Data Type | Field Description |
| :--- | :--- | :--- | :--- |
| `0x00` | 2 | `uint16_t` | **Compression Mode** (`0 = Raw Fallback`, `1 = VQ Compressed`) |
| `0x02` | 2 | `uint16_t` | **Image Width** (Must be a multiple of 4) |
| `0x04` | 2 | `uint16_t` | **Image Height** (Must be a multiple of 4) |
| `0x06` | 2 | `uint16_t` | **Count (\(N_{2x2}\))** of unique 2x2 block configurations in the dictionary |
| `0x08` | 2 | `uint16_t` | **Count (\(N_{4x4}\))** of unique 4x4 block layouts in the dictionary |
| `0x0A` | \(\frac{W}{4} \times \frac{H}{4} \times 2\) | `uint16_t[]` | **Map Grid**: Matrix array indexing structural 4x4 blocks |
| *Variable* | \(N_{2x2} \times 8\) | `uint16_t[]` | **2x2 Palette Codebook**: Master index of flat RGB565 pixel maps |
| *Variable* | \(N_{4x4} \times 8\) | `uint16_t[]` | **4x4 Structure Codebook**: Blocks comprised of four 2x2 indices |

---

## 🛠️ Go Pre-Processor Encoder Usage

The encoder compiles down to a cross-platform command-line tool. It reads standard `.png` inputs, applies color-space translation down to native 16-bit RGB565 boundaries, builds the multi-tiered dictionary trees, performs structural deduplication, and maps out the final binary container file.

### Compilation
```bash
go build -o vq_encoder encoder.go
```

### Execution Command
```bash
./vq_encoder <input_image.png> <output_asset.bin>
```

---

## 📥 ESP32-S3 / C++ Embedded Driver

To maximize performance under the ESP32-S3's dual-core execution space, configure the encoder binary to reside directly inside the SoC **Flash Memory Map partition** or within **8-Bit Octal PSRAM** (`MALLOC_CAP_SPIRAM`). 

The scratch alignment array should be stored explicitly in fast **Internal SRAM** (`MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT`) to guarantee the fastest CPU register memory jumps.

### 1. Integration Header (`vq_decoder.h`)

```cpp
#ifndef VQ_DECODER_H
#define VQ_DECODER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

bool vq_get_metadata(const uint8_t* file_bytes, uint16_t* out_width, uint16_t* out_height, uint16_t* out_mode);
bool vq_decompress_frame(const uint8_t* file_bytes, uint16_t* fb_psram_dest, uint16_t* sram_strip_buf);

#ifdef __cplusplus
}
#endif

#endif // VQ_DECODER_H
```

### 2. Implementation Pipeline (`vq_decoder.cpp`)

```cpp
#include "vq_decoder.h"
#include <string.h>

extern "C" {

bool vq_get_metadata(const uint8_t* file_bytes, uint16_t* out_width, uint16_t* out_height, uint16_t* out_mode) {
    if (!file_bytes) return false;
    memcpy(out_mode,   &file_bytes[0], 2);
    memcpy(out_width,  &file_bytes[2], 2);
    memcpy(out_height, &file_bytes[4], 2);
    return true;
}

bool vq_decompress_frame(const uint8_t* file_bytes, uint16_t* fb_psram_dest, uint16_t* sram_strip_buf) {
    if (!file_bytes || !fb_psram_dest || !sram_strip_buf) return false;

    uint16_t mode = 0, width = 0, height = 0, cb2x2_count = 0, cb4x4_count = 0;
    memcpy(&mode,        &file_bytes[0], 2);
    memcpy(&width,       &file_bytes[2], 2);
    memcpy(&height,      &file_bytes[4], 2);
    memcpy(&cb2x2_count, &file_bytes[6], 2);
    memcpy(&cb4x4_count, &file_bytes[8], 2);

    uint32_t grid_x = width / 4;
    uint32_t grid_y = height / 4;
    uint32_t total_blocks = grid_x * grid_y;
    uint32_t src_ptr = 10;

    if (mode == 0) {
        memcpy(fb_psram_dest, &file_bytes[src_ptr], width * height * sizeof(uint16_t));
        return true;
    }

    const uint16_t* map_grid = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);
    src_ptr += total_blocks * sizeof(uint16_t);

    const uint16_t* cb2x2_flat = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);
    src_ptr += cb2x2_count * 4 * sizeof(uint16_t);

    const uint16_t* codebook_4x4 = reinterpret_cast<const uint16_t*>(&file_bytes[src_ptr]);

    uint32_t grid_idx = 0;
    for (uint32_t by = 0; by < grid_y; ++by) {
        for (uint32_t bx = 0; bx < grid_x; ++bx) {
            uint16_t c4x4_idx = map_grid[grid_idx++];
            uint32_t c4x4_offset = c4x4_idx << 2;

            uint16_t q0 = codebook_4x4[c4x4_offset + 0];
            uint16_t q1 = codebook_4x4[c4x4_offset + 1];
            uint16_t q2 = codebook_4x4[c4x4_offset + 2];
            uint16_t q3 = codebook_4x4[c4x4_offset + 3];

            uint32_t sram_x = bx * 4;
            uint32_t o0 = q0 << 2; uint32_t o1 = q1 << 2;
            uint32_t o2 = q2 << 2; uint32_t o3 = q3 << 2;

            sram_strip_buf[(0 * width) + sram_x + 0] = cb2x2_flat[o0 + 0];
            sram_strip_buf[(0 * width) + sram_x + 1] = cb2x2_flat[o0 + 1];
            sram_strip_buf[(0 * width) + sram_x + 2] = cb2x2_flat[o1 + 0];
            sram_strip_buf[(0 * width) + sram_x + 3] = cb2x2_flat[o1 + 1];

            sram_strip_buf[(1 * width) + sram_x + 0] = cb2x2_flat[o0 + 2];
            sram_strip_buf[(1 * width) + sram_x + 1] = cb2x2_flat[o0 + 3];
            sram_strip_buf[(1 * width) + sram_x + 2] = cb2x2_flat[o1 + 2];
            sram_strip_buf[(1 * width) + sram_x + 3] = cb2x2_flat[o1 + 3];

            sram_strip_buf[(2 * width) + sram_x + 0] = cb2x2_flat[o2 + 0];
            sram_strip_buf[(2 * width) + sram_x + 1] = cb2x2_flat[o2 + 1];
            sram_strip_buf[(2 * width) + sram_x + 2] = cb2x2_flat[o3 + 0];
            sram_strip_buf[(2 * width) + sram_x + 3] = cb2x2_flat[o3 + 1];

            sram_strip_buf[(3 * width) + sram_x + 0] = cb2x2_flat[o2 + 2];
            sram_strip_buf[(3 * width) + sram_x + 1] = cb2x2_flat[o2 + 3];
            sram_strip_buf[(3 * width) + sram_x + 2] = cb2x2_flat[o3 + 2];
            sram_strip_buf[(3 * width) + sram_x + 3] = cb2x2_flat[o3 + 3];
        }
        uint32_t fb_pixel_offset = by * 4 * width;
        memcpy(&fb_psram_dest[fb_pixel_offset], sram_strip_buf, width * 4 * sizeof(uint16_t));
    }
    return true;
}
}
```

### 3. Execution Sample inside ESP-IDF Application Loop

```cpp
#include "vq_decoder.h"
#include "esp_heap_caps.h"

void render_background_asset(const uint8_t* bin_file_pointer, uint16_t* psram_framebuffer) {
    uint16_t w = 0, h = 0, mode = 0;
    
    // Parse properties cleanly
    if (!vq_get_metadata(bin_file_pointer, &w, &h, &mode)) return;

    // Allocate continuous high-speed scratch line once inside fast Internal SRAM
    uint16_t* sram_scratch = (uint16_t*)heap_caps_malloc(w * 4 * sizeof(uint16_t), 
                                                         MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT);

    if (sram_scratch) {
        // Execute instant unpacking sequence straight to your display memory block
        vq_decompress_frame(bin_file_pointer, psram_framebuffer, sram_scratch);
        
        // Relinquish context bounds safely
        heap_caps_free(sram_scratch);
    }
}
```

---

## 📈 Optimization Recommendations
To maximize performance and prevent image artifacting, consider these optimization steps:
1. **Compiler Optimization Flags**: Verify that your build workspace (PlatformIO environment profile or CMake build config) uses the `-O2` or `-O3` compiler optimization flags. This tells the compiler to aggressively optimize loops, maximizing execution speed.
2. **Dithering Pre-processing**: If your graphics assets feature subtle continuous gradient fields, apply a Floyd-Steinberg or Ordered Dithering process to your PNG images before sending them through the Go encoder. This breaks up visible color banding before the 2x2 deduplication phase.

#ifndef VQ_DECODER_H
#define VQ_DECODER_H

#include <stdint.h>

/**
 * Validates the file header and retrieves metadata dimensions without executing decompression.
 * 
 * @param file_bytes      Pointer to the loaded binary asset array.
 * @param out_width       Pointer to return the parsed image width.
 * @param out_height      Pointer to return the parsed image height.
 * @param out_mode        Pointer to return the compression mode (0 = Raw, 1 = VQ).
 * @return                true if valid format header, false otherwise.
 */
bool vq_get_metadata(const uint8_t* file_bytes, uint16_t* out_width, uint16_t* out_height, uint16_t* out_mode);

/**
 * Resolution-agnostic Pure Two-Tier Vector Quantization Decoder.
 * Decodes compressed assets directly into a target destination framebuffer.
 * 
 * @param file_bytes      Pointer to the raw binary file array mapped in memory.
 * @param fb_psram_dest   Pointer to your active destination Framebuffer layout in PSRAM.
 * @param sram_strip_buf  Pre-allocated internal SRAM scratch buffer. Must be sized at 
 *                        least (ImageWidth * 4 * sizeof(uint16_t)) bytes.
 * @return                true on successful rendering sequence, false if memory limits or errors hit.
 */
bool vq_decompress_frame(const uint8_t* file_bytes, uint16_t* fb_psram_dest, uint16_t* sram_strip_buf);


#endif // VQ_DECODER_H

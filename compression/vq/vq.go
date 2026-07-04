package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	_ "image/png"
	"os"
)

// 2x2 pixels block structure
type Block2x2 [2][2]uint16

// 4x4 block structure, represented by 4 indices pointing to 2x2 blocks
type Block4x4 [4]uint16

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <input.png> <output.bin>")
		return
	}

	command := os.Args[1]

	inputPath := os.Args[2]
	outputPath := os.Args[3]

	switch command {
	case "encode":
		VqEncode(inputPath, outputPath)
	case "decode":
		VqDecode()
	default:
		fmt.Println("Unknown command. Use 'encode' or 'decode'.")
	}
}

func VqEncode(inputPath, outputPath string) {

	// 1. Load the PNG File
	img, err := loadPNG(inputPath)
	if err != nil {
		fmt.Printf("Error loading PNG: %v\n", err)
		return
	}

	Width := img.Bounds().Dx()
	Height := img.Bounds().Dy()
	RawSize := Width * Height * 2 // 460800 bytes
	GridX := Width / 4            // 120 blocks
	GridY := Height / 4           // 120 blocks
	MapCount := GridX * GridY     // 14400 entries

	// 2. Convert to raw 480x480 RGB565 pixel matrix
	pixels := convertToRGB565(img)

	// 3. Build & Deduplicate 2x2 Codebook
	codebook2x2 := make([]Block2x2, 0)
	map2x2ToIdx := make(map[Block2x2]uint16)

	get2x2Index := func(b Block2x2) uint16 {
		if idx, found := map2x2ToIdx[b]; found {
			return idx
		}
		idx := uint16(len(codebook2x2))
		codebook2x2 = append(codebook2x2, b)
		map2x2ToIdx[b] = idx
		return idx
	}

	// 4. Trace the image grid by 4x4 steps
	mapGrid := make([]uint16, MapCount)

	codebook4x4 := make([]Block4x4, 0)
	map4x4ToIdx := make(map[Block4x4]uint16)

	gridIdx := 0
	for by := 0; by < GridY; by++ {
		for bx := 0; bx < GridX; bx++ {
			pixelX := bx * 4
			pixelY := by * 4

			// Extract 4 sub-blocks of 2x2 sizes inside the 4x4 perimeter
			var b0, b1, b2, b3 Block2x2
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					b0[y][x] = pixels[(pixelY+y)*Width+pixelX+x]
					b1[y][x] = pixels[(pixelY+y)*Width+pixelX+2+x]
					b2[y][x] = pixels[(pixelY+2+y)*Width+pixelX+x]
					b3[y][x] = pixels[(pixelY+2+y)*Width+pixelX+2+x]
				}
			}

			// Gather their respective 2x2 IDs
			var current4x4 Block4x4
			current4x4[0] = get2x2Index(b0)
			current4x4[1] = get2x2Index(b1)
			current4x4[2] = get2x2Index(b2)
			current4x4[3] = get2x2Index(b3)

			// Look up if this exact 4x4 composition layout has already been recorded
			if cachedIdx, found := map4x4ToIdx[current4x4]; found {
				// Flag = 0 (MSB is 0). The value points directly into our 4x4 dictionary
				mapGrid[gridIdx] = cachedIdx
			} else {
				// This is a brand new unique 4x4 block configuration
				// Record it into the dictionary so future matching 4x4 blocks can point to it
				map4x4ToIdx[current4x4] = uint16(len(codebook4x4))
				codebook4x4 = append(codebook4x4, current4x4)

				// Flag = 1 (MSB is 1). Tells decoder to check the inline stream for 4 blocks
				mapGrid[gridIdx] = uint16(gridIdx)
			}
			gridIdx++
		}
	}

	// 5. Size Evaluation / Budget Balancing
	// Map Grid: 14,400 entries * 2 bytes = 28,800 bytes
	// 2x2 Codebook: count * 8 bytes (4 pixels * 2 bytes)
	// 4x4 Codebook: count * 8 bytes (4 indexes * 2 bytes)
	// Unique Stream: count * 2 bytes
	sizeMapGrid := len(mapGrid) * 2
	sizeCodebook2x2 := len(codebook2x2) * 8
	sizeCodebook4x4 := len(codebook4x4) * 8

	compressedPayloadSize := sizeMapGrid + sizeCodebook2x2 + sizeCodebook4x4
	totalCompressedFileBytes := 4 + compressedPayloadSize // Adding 4-byte descriptor header

	fmt.Printf("--- Analysis Results ---\n")
	fmt.Printf("Unique 2x2 Blocks: %d (%d bytes)\n", len(codebook2x2), sizeCodebook2x2)
	fmt.Printf("Unique 4x4 Blocks: %d (%d bytes)\n", len(codebook4x4), sizeCodebook4x4)
	fmt.Printf("Total Compressed Budget: %d bytes\n", totalCompressedFileBytes)
	fmt.Printf("Raw Frame Size Budget:   %d bytes\n", RawSize)

	// Guard check against overflowing index bounds
	if len(codebook4x4) > 0x7FFF || len(codebook2x2) > 0xFFFF {
		fmt.Println("Warning: Codebook limits exceeded. Forcing raw fallback mode.")
		saveRawFallback(pixels, Width, Height, outputPath)
		return
	}

	if totalCompressedFileBytes >= RawSize {
		fmt.Println("Result: Compression does not save space. Writing raw fallback binary.")
		saveRawFallback(pixels, Width, Height, outputPath)
	} else {
		fmt.Printf("Result: Success! Generating custom binary of %d bytes.\n", totalCompressedFileBytes)
		saveCompressedFormat(outputPath, Width, Height, mapGrid, codebook2x2, codebook4x4)
	}
}

// Loads image from disk and converts it to basic object model
func loadPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// Transforms standard system colors into target 16-bit RGB565 pixels
func convertToRGB565(img image.Image) []uint16 {
	Width := img.Bounds().Dx()
	Height := img.Bounds().Dy()

	matrix := make([]uint16, Width*Height)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Downscale 16-bit color bounds (0-65535) down to 5-6-5 bits respectively
			r5 := uint16(r>>11) & 0x1F
			g6 := uint16(g>>10) & 0x3F
			b5 := uint16(b>>11) & 0x1F
			matrix[y*Width+x] = (r5 << 11) | (g6 << 5) | b5
		}
	}
	return matrix
}

// +-------------------+-------------------+-----------------------------------+
//
// | Offset            | Size              | Content                           |
// +-------------------+-------------------+-----------------------------------+
//
// | 0x00              | 2 Byte            | Compression Mode (0=Raw, 1=VQ)    |
// | 0x02              | 2 Bytes           | Width                             |
// | 0x04              | 2 Bytes           | Height                            |
// | 0x06              | 2 Bytes           | Count of unique 2x2 Blocks        |
// | 0x08              | 2 Bytes           | Count of unique 4x4 Blocks        |
// | 0x0A              | W/4 * H/4 * 2     | Map Grid (uint16 arrays)          |
// | Variable          | Count * 8 Bytes   | 2x2 Pixel Codebook (RGB565)       |
// | Variable          | Variable          | 4x4 Index Dictionary              |
// +-------------------+-------------------+-----------------------------------+

// Save Output Strategy 0: Raw RGB565 File Output
func saveRawFallback(pixels []uint16, width, height int, path string) {
	buf := new(bytes.Buffer)

	// Header description bytes: 0x00 signifies raw uncompressed sequence
	buf.WriteByte(0) // Mode = 0
	buf.WriteByte(0) // Padding
	buf.WriteByte(byte(width & 0xFF))
	buf.WriteByte(byte((width >> 8) & 0xFF))
	buf.WriteByte(byte(height & 0xFF))
	buf.WriteByte(byte((height >> 8) & 0xFF))
	buf.WriteByte(0) // Padding
	buf.WriteByte(0) // Padding

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			binary.Write(buf, binary.LittleEndian, pixels[y*width+x])
		}
	}
	_ = os.WriteFile(path, buf.Bytes(), 0644)
}

// Save Output Strategy 1: Tailored Binary Container Layout
func saveCompressedFormat(path string, width, height int, grid []uint16, cb2x2 []Block2x2, cb4x4 []Block4x4) {
	buf := new(bytes.Buffer)

	// 1. Header Segment (4 bytes)
	buf.WriteByte(1) // Mode = 1 (Compressed format indicator)
	buf.WriteByte(0) // Padding byte

	// Width and Height
	buf.WriteByte(byte(width & 0xFF))
	buf.WriteByte(byte((width >> 8) & 0xFF))
	buf.WriteByte(byte(height & 0xFF))
	buf.WriteByte(byte((height >> 8) & 0xFF))

	unique2x2Count := uint16(len(cb2x2))
	buf.WriteByte(byte(unique2x2Count & 0xFF))
	buf.WriteByte(byte((unique2x2Count >> 8) & 0xFF))

	unique4x4Count := uint16(len(cb4x4))
	buf.WriteByte(byte(unique4x4Count & 0xFF))
	buf.WriteByte(byte((unique4x4Count >> 8) & 0xFF))

	// Store how many 2x2 dictionary nodes must be allocated in ESP32-S3 internal SRAM
	binary.Write(buf, binary.LittleEndian, unique2x2Count)

	// 2. Map Grid Section ((width/4) * (height/4) entries * 2 bytes)
	for _, val := range grid {
		binary.Write(buf, binary.LittleEndian, val)
	}

	// 3. Write 2x2 Master Codebook
	for _, block := range cb2x2 {
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				binary.Write(buf, binary.LittleEndian, block[y][x])
			}
		}
	}

	// 4. Write 4x4 Codebook References
	for _, block := range cb4x4 {
		for i := 0; i < 4; i++ {
			binary.Write(buf, binary.LittleEndian, block[i])
		}
	}

	_ = os.WriteFile(path, buf.Bytes(), 0644)
}

func VqDecode() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run decoder.go <input.bin> <output.png>")
		return
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	// 1. Read file into a binary buffer reader
	fileData, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading binary file: %v\n", err)
		return
	}
	reader := bytes.NewReader(fileData)

	// 2. Parse 4-byte Descriptor Header
	mode, err := reader.ReadByte()
	if err != nil {
		fmt.Println("Error reading format mode byte")
		return
	}
	_, _ = reader.ReadByte() // Read over padding byte

	var Value uint16
	if err := binary.Read(reader, binary.LittleEndian, &Value); err != nil {
		fmt.Println("Error reading width")
		return
	}
	Width := int(Value)

	if err := binary.Read(reader, binary.LittleEndian, &Value); err != nil {
		fmt.Println("Error reading height")
		return
	}
	Height := int(Value)

	if err := binary.Read(reader, binary.LittleEndian, &Value); err != nil {
		fmt.Println("Error reading codebook 2x2 count")
		return
	}
	cb2x2Count := int(Value)

	if err := binary.Read(reader, binary.LittleEndian, &Value); err != nil {
		fmt.Println("Error reading codebook 4x4 count")
		return
	}
	cb4x4Count := int(Value)

	GridX := Width / 4        // 120 blocks
	GridY := Height / 4       // 120 blocks
	MapCount := GridX * GridY // 14400 entries

	pixels := make([]uint16, Width*Height)

	fmt.Printf("--- Parsing Header ---\n")
	if mode == 0 {
		fmt.Println("Detected Mode: 0 (Raw Fallback RGB565)")
		// Decompress Strategy 0: Direct Sequential Read
		for y := 0; y < Height; y++ {
			for x := 0; x < Width; x++ {
				if err := binary.Read(reader, binary.LittleEndian, &pixels[y*Width+x]); err != nil {
					fmt.Printf("Unexpected EOF reading raw pixels at (%d, %d)\n", x, y)
					return
				}
			}
		}
	} else if mode == 1 {
		fmt.Printf("Detected Mode: 1 (Double-Layer VQ Compressed)\n")
		fmt.Printf("Expecting Unique 2x2 Codebook Nodes: %d\n", cb2x2Count)

		// 3. Read Map Grid (120x120 entries)
		mapGrid := make([]uint16, MapCount)
		for i := 0; i < len(mapGrid); i++ {
			if err := binary.Read(reader, binary.LittleEndian, &mapGrid[i]); err != nil {
				fmt.Println("Error parsing Map Grid entries")
				return
			}
		}

		// 4. Read 2x2 Master Palette Codebook
		codebook2x2 := make([]Block2x2, cb2x2Count)
		for i := 0; i < int(cb2x2Count); i++ {
			for y := 0; y < 2; y++ {
				for x := 0; x < 2; x++ {
					if err := binary.Read(reader, binary.LittleEndian, &codebook2x2[i][y][x]); err != nil {
						fmt.Println("Error reading 2x2 codebook contents")
						return
					}
				}
			}
		}

		// 5. Read 4x4 Index Dictionary Codebook
		codebook4x4 := make([]Block4x4, cb4x4Count)
		for i := 0; i < cb4x4Count; i++ {
			for j := 0; j < 4; j++ {
				if err := binary.Read(reader, binary.LittleEndian, &codebook4x4[i][j]); err != nil {
					fmt.Println("Error reading 4x4 codebook indices")
					return
				}
			}
		}

		// 6. Execute Hierarchical Processing (Reconstruct the image grid)
		gridIdx := 0
		for by := 0; by < GridY; by++ {
			for bx := 0; bx < GridX; bx++ {
				blockCmd := mapGrid[gridIdx]
				gridIdx++

				c4x4Idx := blockCmd & 0x7FFF
				q0 := codebook4x4[c4x4Idx][0]
				q1 := codebook4x4[c4x4Idx][1]
				q2 := codebook4x4[c4x4Idx][2]
				q3 := codebook4x4[c4x4Idx][3]

				// Map the sub-quadrants directly out to the 480x480 pixel frame destination
				pixelX := bx * 4
				pixelY := by * 4

				// Map Top-Left (q0) and Top-Right (q1)
				for y := 0; y < 2; y++ {
					for x := 0; x < 2; x++ {
						pixels[(pixelY+y)*Width+pixelX+x] = codebook2x2[q0][y][x]
						pixels[(pixelY+y)*Width+pixelX+2+x] = codebook2x2[q1][y][x]
					}
				}
				// Map Bottom-Left (q2) and Bottom-Right (q3)
				for y := 0; y < 2; y++ {
					for x := 0; x < 2; x++ {
						pixels[(pixelY+2+y)*Width+pixelX+x] = codebook2x2[q2][y][x]
						pixels[(pixelY+2+y)*Width+pixelX+2+x] = codebook2x2[q3][y][x]
					}
				}
			}
		}
		fmt.Println("Decompression completed successfully!")
	} else {
		fmt.Printf("Error: Unknown decompression header mode (%d)\n", mode)
		return
	}

	// 7. Output to a PNG image file
	err = savePNG(outputPath, Width, Height, pixels)
	if err != nil {
		fmt.Printf("Error writing target PNG: %v\n", err)
	} else {
		fmt.Printf("Successfully exported image back to: %s\n", outputPath)
	}
}

// Converts 16-bit RGB565 matrix back into a modern 32-bit RGBA PNG
func savePNG(path string, width, height int, pixels []uint16) error {
	upImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p565 := pixels[y*width+x]

			// Extract RGB bits from standard bitmask boundaries
			r5 := uint8((p565 >> 11) & 0x1F)
			g6 := uint8((p565 >> 5) & 0x3F)
			b5 := uint8(p565 & 0x1F)

			// Upscale channels cleanly from 5/6 bits back up to full standard 8-bit space (0-255)
			r8 := (r5 << 3) | (r5 >> 2)
			g8 := (g6 << 2) | (g6 >> 4)
			b8 := (b5 << 3) | (b5 >> 2)

			upImg.SetRGBA(x, y, color.RGBA{R: r8, G: g8, B: b8, A: 255})
		}
	}

	outImgFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outImgFile.Close()

	return png.Encode(outImgFile, upImg)
}

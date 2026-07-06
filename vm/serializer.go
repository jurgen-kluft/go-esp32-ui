package vm

import (
	"encoding/binary"
	"fmt"
	"sort"
)

const (
	programImageVersion uint16 = 1

	programImageHeaderSize      = 36
	programImageBlockRecordSize = 32
	programImageConstRecordSize = 20

	programConstStorageInline  uint8 = 0
	programConstStoragePayload uint8 = 1
)

var programImageMagic = [4]byte{'G', 'V', 'M', '1'}

type ProgramImageHeader struct {
	Magic            [4]byte
	Version          uint16
	HeaderSize       uint16
	TotalSize        uint32
	EntryBlockID     uint32
	BlockCount       uint32
	BlockTableOffset uint32
	StringDataOffset uint32
	StringDataSize   uint32
	Reserved         uint32
}

type ProgramImageBlockRecord struct {
	BlockID          uint32
	Scope            uint8
	Reserved0        uint8
	Reserved1        uint16
	InheritedLocals  uint32
	LocalCount       uint32
	BytecodeOffset   uint32
	BytecodeSize     uint32
	ConstTableOffset uint32
	ConstCount       uint32
}

type ProgramImageConstRecord struct {
	ConstIndex    uint32
	Type          uint8
	Flags         uint8
	Storage       uint8
	Reserved      uint8
	InlineValue   uint32
	PayloadOffset uint32
	PayloadSize   uint32
}

type programImageString []byte

type plannedProgramImageBlock struct {
	record ProgramImageBlockRecord
	consts []ProgramImageConstRecord
	bytes  []byte
}

func align4(value uint32) uint32 {
	return (value + 3) &^ 3
}

func putU16(dst []byte, value uint16) {
	binary.LittleEndian.PutUint16(dst, value)
}

func putU32(dst []byte, value uint32) {
	binary.LittleEndian.PutUint32(dst, value)
}

func readU16(src []byte) uint16 {
	return binary.LittleEndian.Uint16(src)
}

func readU32(src []byte) uint32 {
	return binary.LittleEndian.Uint32(src)
}

func readProgramImageHeader(data []byte) (ProgramImageHeader, error) {
	if len(data) < programImageHeaderSize {
		return ProgramImageHeader{}, fmt.Errorf("program image too small: got %d want at least %d", len(data), programImageHeaderSize)
	}

	header := ProgramImageHeader{}
	copy(header.Magic[:], data[:4])
	header.Version = readU16(data[4:6])
	header.HeaderSize = readU16(data[6:8])
	header.TotalSize = readU32(data[8:12])
	header.EntryBlockID = readU32(data[12:16])
	header.BlockCount = readU32(data[16:20])
	header.BlockTableOffset = readU32(data[20:24])
	header.StringDataOffset = readU32(data[24:28])
	header.StringDataSize = readU32(data[28:32])
	header.Reserved = readU32(data[32:36])
	return header, nil
}

func readProgramImageBlockRecord(data []byte, offset uint32) (ProgramImageBlockRecord, error) {
	end := offset + programImageBlockRecordSize
	if end > uint32(len(data)) {
		return ProgramImageBlockRecord{}, fmt.Errorf("block record out of bounds: offset=%d size=%d total=%d", offset, programImageBlockRecordSize, len(data))
	}

	src := data[offset:end]
	record := ProgramImageBlockRecord{}
	record.BlockID = readU32(src[0:4])
	record.Scope = src[4]
	record.Reserved0 = src[5]
	record.Reserved1 = readU16(src[6:8])
	record.InheritedLocals = readU32(src[8:12])
	record.LocalCount = readU32(src[12:16])
	record.BytecodeOffset = readU32(src[16:20])
	record.BytecodeSize = readU32(src[20:24])
	record.ConstTableOffset = readU32(src[24:28])
	record.ConstCount = readU32(src[28:32])
	return record, nil
}

func readProgramImageConstRecord(data []byte, offset uint32) (ProgramImageConstRecord, error) {
	end := offset + programImageConstRecordSize
	if end > uint32(len(data)) {
		return ProgramImageConstRecord{}, fmt.Errorf("const record out of bounds: offset=%d size=%d total=%d", offset, programImageConstRecordSize, len(data))
	}

	src := data[offset:end]
	record := ProgramImageConstRecord{}
	record.ConstIndex = readU32(src[0:4])
	record.Type = src[4]
	record.Flags = src[5]
	record.Storage = src[6]
	record.Reserved = src[7]
	record.InlineValue = readU32(src[8:12])
	record.PayloadOffset = readU32(src[12:16])
	record.PayloadSize = readU32(src[16:20])
	return record, nil
}

func validateImageRegion(totalSize uint32, offset uint32, size uint32, label string) error {
	if offset > totalSize {
		return fmt.Errorf("%s offset %d outside image size %d", label, offset, totalSize)
	}
	if size > totalSize-offset {
		return fmt.Errorf("%s region [%d:%d] outside image size %d", label, offset, offset+size, totalSize)
	}
	return nil
}

func encodeHeader(data []byte, header ProgramImageHeader) {
	copy(data[0:4], header.Magic[:])
	putU16(data[4:6], header.Version)
	putU16(data[6:8], header.HeaderSize)
	putU32(data[8:12], header.TotalSize)
	putU32(data[12:16], header.EntryBlockID)
	putU32(data[16:20], header.BlockCount)
	putU32(data[20:24], header.BlockTableOffset)
	putU32(data[24:28], header.StringDataOffset)
	putU32(data[28:32], header.StringDataSize)
	putU32(data[32:36], header.Reserved)
}

func encodeBlockRecord(data []byte, offset uint32, record ProgramImageBlockRecord) {
	dst := data[offset : offset+programImageBlockRecordSize]
	putU32(dst[0:4], record.BlockID)
	dst[4] = record.Scope
	dst[5] = record.Reserved0
	putU16(dst[6:8], record.Reserved1)
	putU32(dst[8:12], record.InheritedLocals)
	putU32(dst[12:16], record.LocalCount)
	putU32(dst[16:20], record.BytecodeOffset)
	putU32(dst[20:24], record.BytecodeSize)
	putU32(dst[24:28], record.ConstTableOffset)
	putU32(dst[28:32], record.ConstCount)
}

func encodeConstRecord(data []byte, offset uint32, record ProgramImageConstRecord) {
	dst := data[offset : offset+programImageConstRecordSize]
	putU32(dst[0:4], record.ConstIndex)
	dst[4] = record.Type
	dst[5] = record.Flags
	dst[6] = record.Storage
	dst[7] = record.Reserved
	putU32(dst[8:12], record.InlineValue)
	putU32(dst[12:16], record.PayloadOffset)
	putU32(dst[16:20], record.PayloadSize)
}

func encodeConstRecordFromVar(variable Var) (ProgramImageConstRecord, string, error) {
	record := ProgramImageConstRecord{
		ConstIndex: uint32(variable.Index),
		Type:       uint8(variable.Type),
		Flags:      uint8(variable.Flags),
	}

	switch variable.Type {
	case VarTypeU8, VarTypeU16, VarTypeU32, VarTypeS8, VarTypeS16, VarTypeS32, VarTypeF32, VarTypeBool:
		record.Storage = programConstStorageInline
		record.InlineValue = variable.Uint32Value()
		return record, "", nil
	case VarTypeStr:
		literal, ok := variable.Value.(string)
		if !ok {
			return ProgramImageConstRecord{}, "", fmt.Errorf("const %d expected string value, got %T", variable.Index, variable.Value)
		}
		record.Storage = programConstStoragePayload
		return record, literal, nil
	default:
		return ProgramImageConstRecord{}, "", fmt.Errorf("const %d has unsupported type %d", variable.Index, variable.Type)
	}
}

func decodeConstRecord(record ProgramImageConstRecord, data []byte, stringStart uint32, stringEnd uint32) (Var, error) {
	variable := Var{
		Index: uint16(record.ConstIndex),
		Type:  VarType(record.Type),
		Flags: VarFlag(record.Flags),
	}

	switch record.Storage {
	case programConstStorageInline:
		variable.SetUint32Value(record.InlineValue)
		return variable, nil
	case programConstStoragePayload:
		if variable.Type != VarTypeStr {
			return Var{}, fmt.Errorf("const %d uses payload storage with unsupported type %d", record.ConstIndex, variable.Type)
		}
		if record.PayloadOffset < stringStart {
			return Var{}, fmt.Errorf("const %d payload starts before string region", record.ConstIndex)
		}
		if record.PayloadOffset > stringEnd {
			return Var{}, fmt.Errorf("const %d payload offset %d outside string region", record.ConstIndex, record.PayloadOffset)
		}
		if record.PayloadSize > stringEnd-record.PayloadOffset {
			return Var{}, fmt.Errorf("const %d payload [%d:%d] outside string region", record.ConstIndex, record.PayloadOffset, record.PayloadOffset+record.PayloadSize)
		}
		variable.Value = programImageString(data[record.PayloadOffset : record.PayloadOffset+record.PayloadSize])
		return variable, nil
	default:
		return Var{}, fmt.Errorf("const %d has unsupported storage %d", record.ConstIndex, record.Storage)
	}
}

func validatePackedVarRef(block ProgramImageBlockRecord, raw uint32, blockIDs map[uint32]struct{}, constCount uint32) error {
	ref := UnpackVarRef(raw)
	switch ref.Storage {
	case GlobalRefType:
		return nil
	case ConstRefType:
		if ref.Index >= constCount {
			return fmt.Errorf("block %d const ref %d out of range %d", block.BlockID, ref.Index, constCount)
		}
		return nil
	case LocalRefType:
		if ref.Index >= block.LocalCount {
			return fmt.Errorf("block %d local ref %d out of range %d", block.BlockID, ref.Index, block.LocalCount)
		}
		return nil
	case TempRefType:
		return nil
	default:
		return fmt.Errorf("block %d uses unknown storage class %d", block.BlockID, ref.Storage)
	}
}

func validateBlockBytecode(block ProgramImageBlockRecord, bytes []byte, blockIDs map[uint32]struct{}) error {
	pc := uint32(0)
	for pc < uint32(len(bytes)) {
		op := Opcode(bytes[pc])
		pc++

		switch op {
		case OpReturn:
			if pc+1 > uint32(len(bytes)) {
				return fmt.Errorf("block %d truncated OpReturn", block.BlockID)
			}
			pc++
		case OpPushVar, OpPopVar, OpGetLocal, OpSetLocal:
			if pc+4 > uint32(len(bytes)) {
				return fmt.Errorf("block %d truncated opcode %d", block.BlockID, op)
			}
			raw := readU32(bytes[pc : pc+4])
			if err := validatePackedVarRef(block, raw, blockIDs, block.ConstCount); err != nil {
				return err
			}
			pc += 4
		case OpAdd, OpSub, OpMul, OpDiv, OpEQ, OpG, OpGE, OpL, OpLE:
		case OpIf:
			if pc+12 > uint32(len(bytes)) {
				return fmt.Errorf("block %d truncated OpIf", block.BlockID)
			}
			for i := 0; i < 3; i++ {
				target := readU32(bytes[pc : pc+4])
				if _, ok := blockIDs[target]; !ok {
					return fmt.Errorf("block %d references missing block %d", block.BlockID, target)
				}
				pc += 4
			}
		case OpSyscall:
			if pc+2 > uint32(len(bytes)) {
				return fmt.Errorf("block %d truncated OpSyscall", block.BlockID)
			}
			pc += 2
		default:
			return fmt.Errorf("block %d uses unknown opcode %d", block.BlockID, op)
		}
	}

	if pc != uint32(len(bytes)) {
		return fmt.Errorf("block %d bytecode ended at %d want %d", block.BlockID, pc, len(bytes))
	}
	return nil
}

func (c *Compiler) WriteProgramImage(entryBlockID uint32) ([]byte, error) {
	if _, ok := c.blocks[entryBlockID]; !ok {
		return nil, fmt.Errorf("entry block %d not found", entryBlockID)
	}

	blockIDs := make([]uint32, 0, len(c.blocks))
	for blockID := range c.blocks {
		blockIDs = append(blockIDs, blockID)
	}
	sort.Slice(blockIDs, func(i, j int) bool { return blockIDs[i] < blockIDs[j] })

	planned := make([]plannedProgramImageBlock, 0, len(blockIDs))
	stringOffsets := make(map[string]uint32)
	stringBytes := make([]byte, 0)

	constRegionOffset := uint32(programImageHeaderSize + len(blockIDs)*programImageBlockRecordSize)
	constRegionCursor := constRegionOffset

	for _, blockID := range blockIDs {
		block := c.blocks[blockID]
		if block.LocalCount < block.InheritedLocals {
			return nil, fmt.Errorf("block %d local metadata invalid: local_count=%d inherited=%d", block.ID, block.LocalCount, block.InheritedLocals)
		}

		record := ProgramImageBlockRecord{
			BlockID:          block.ID,
			Scope:            uint8(block.Scope),
			InheritedLocals:  block.InheritedLocals,
			LocalCount:       block.LocalCount,
			BytecodeSize:     uint32(len(block.Bytes)),
			ConstCount:       uint32(len(block.Consts)),
			ConstTableOffset: 0,
		}
		if len(block.Consts) > 0 {
			record.ConstTableOffset = constRegionCursor
			constRegionCursor += uint32(len(block.Consts) * programImageConstRecordSize)
		}

		consts := make([]ProgramImageConstRecord, len(block.Consts))
		for constIndex, variable := range block.Consts {
			if variable.Type == VarTypeNone {
				return nil, fmt.Errorf("block %d const %d is unset", block.ID, constIndex)
			}
			if int(variable.Index) != constIndex {
				return nil, fmt.Errorf("block %d const index mismatch: slice=%d var=%d", block.ID, constIndex, variable.Index)
			}

			constRecord, literal, err := encodeConstRecordFromVar(variable)
			if err != nil {
				return nil, err
			}
			if constRecord.Storage == programConstStoragePayload {
				offset, ok := stringOffsets[literal]
				if !ok {
					offset = uint32(len(stringBytes))
					stringOffsets[literal] = offset
					stringBytes = append(stringBytes, literal...)
				}
				constRecord.PayloadOffset = offset
				constRecord.PayloadSize = uint32(len(literal))
			}
			consts[constIndex] = constRecord
		}

		planned = append(planned, plannedProgramImageBlock{record: record, consts: consts, bytes: block.Bytes})
	}

	stringDataOffset := align4(constRegionCursor)
	stringDataSize := uint32(len(stringBytes))
	bytecodeCursor := align4(stringDataOffset + stringDataSize)
	for i := range planned {
		planned[i].record.BytecodeOffset = bytecodeCursor
		bytecodeCursor += planned[i].record.BytecodeSize
		bytecodeCursor = align4(bytecodeCursor)
		for j := range planned[i].consts {
			if planned[i].consts[j].Storage == programConstStoragePayload {
				planned[i].consts[j].PayloadOffset += stringDataOffset
			}
		}
	}

	totalSize := bytecodeCursor
	data := make([]byte, totalSize)
	header := ProgramImageHeader{
		Magic:            programImageMagic,
		Version:          programImageVersion,
		HeaderSize:       programImageHeaderSize,
		TotalSize:        totalSize,
		EntryBlockID:     entryBlockID,
		BlockCount:       uint32(len(planned)),
		BlockTableOffset: programImageHeaderSize,
		StringDataOffset: stringDataOffset,
		StringDataSize:   stringDataSize,
	}
	encodeHeader(data, header)

	blockTableOffset := uint32(programImageHeaderSize)
	for i, block := range planned {
		recordOffset := blockTableOffset + uint32(i*programImageBlockRecordSize)
		encodeBlockRecord(data, recordOffset, block.record)
		for j, constRecord := range block.consts {
			constOffset := block.record.ConstTableOffset + uint32(j*programImageConstRecordSize)
			encodeConstRecord(data, constOffset, constRecord)
		}
		copy(data[block.record.BytecodeOffset:block.record.BytecodeOffset+block.record.BytecodeSize], block.bytes)
	}
	copy(data[stringDataOffset:stringDataOffset+stringDataSize], stringBytes)

	return data, nil
}

func (vm *VM) LoadProgramImage(data []byte) (uint32, error) {
	header, err := readProgramImageHeader(data)
	if err != nil {
		return 0, err
	}
	if header.Magic != programImageMagic {
		return 0, fmt.Errorf("invalid program image magic %q", header.Magic)
	}
	if header.Version != programImageVersion {
		return 0, fmt.Errorf("unsupported program image version %d", header.Version)
	}
	if header.HeaderSize < programImageHeaderSize {
		return 0, fmt.Errorf("program image header too small: got %d want at least %d", header.HeaderSize, programImageHeaderSize)
	}
	if header.TotalSize != uint32(len(data)) {
		return 0, fmt.Errorf("program image size mismatch: header=%d actual=%d", header.TotalSize, len(data))
	}
	if err := validateImageRegion(header.TotalSize, header.BlockTableOffset, header.BlockCount*programImageBlockRecordSize, "block table"); err != nil {
		return 0, err
	}
	if err := validateImageRegion(header.TotalSize, header.StringDataOffset, header.StringDataSize, "string data"); err != nil {
		return 0, err
	}

	blockRecords := make([]ProgramImageBlockRecord, header.BlockCount)
	blockIDs := make(map[uint32]struct{}, header.BlockCount)
	for i := uint32(0); i < header.BlockCount; i++ {
		offset := header.BlockTableOffset + i*programImageBlockRecordSize
		record, err := readProgramImageBlockRecord(data, offset)
		if err != nil {
			return 0, err
		}
		if record.Scope != uint8(BlockScopeFrame) && record.Scope != uint8(BlockScopeSub) {
			return 0, fmt.Errorf("block %d has invalid scope %d", record.BlockID, record.Scope)
		}
		if record.LocalCount < record.InheritedLocals {
			return 0, fmt.Errorf("block %d local metadata invalid: local_count=%d inherited=%d", record.BlockID, record.LocalCount, record.InheritedLocals)
		}
		if _, exists := blockIDs[record.BlockID]; exists {
			return 0, fmt.Errorf("duplicate block ID %d", record.BlockID)
		}
		blockIDs[record.BlockID] = struct{}{}
		blockRecords[i] = record
	}
	if _, ok := blockIDs[header.EntryBlockID]; !ok {
		return 0, fmt.Errorf("entry block %d not present in image", header.EntryBlockID)
	}

	loadedBlocks := make(map[uint32]VMBlock, len(blockRecords))
	stringStart := header.StringDataOffset
	stringEnd := header.StringDataOffset + header.StringDataSize
	for _, record := range blockRecords {
		if record.ConstCount > 0 {
			if err := validateImageRegion(header.TotalSize, record.ConstTableOffset, record.ConstCount*programImageConstRecordSize, fmt.Sprintf("block %d const table", record.BlockID)); err != nil {
				return 0, err
			}
		}
		if err := validateImageRegion(header.TotalSize, record.BytecodeOffset, record.BytecodeSize, fmt.Sprintf("block %d bytecode", record.BlockID)); err != nil {
			return 0, err
		}

		consts := make([]Var, record.ConstCount)
		for constIndex := uint32(0); constIndex < record.ConstCount; constIndex++ {
			constOffset := record.ConstTableOffset + constIndex*programImageConstRecordSize
			constRecord, err := readProgramImageConstRecord(data, constOffset)
			if err != nil {
				return 0, err
			}
			if constRecord.ConstIndex != constIndex {
				return 0, fmt.Errorf("block %d const order mismatch: got %d want %d", record.BlockID, constRecord.ConstIndex, constIndex)
			}
			variable, err := decodeConstRecord(constRecord, data, stringStart, stringEnd)
			if err != nil {
				return 0, err
			}
			consts[constIndex] = variable
		}

		bytecode := data[record.BytecodeOffset : record.BytecodeOffset+record.BytecodeSize]
		if err := validateBlockBytecode(record, bytecode, blockIDs); err != nil {
			return 0, err
		}

		loadedBlocks[record.BlockID] = VMBlock{
			ID:              record.BlockID,
			Scope:           BlockScope(record.Scope),
			InheritedLocals: record.InheritedLocals,
			LocalCount:      record.LocalCount,
			Bytes:           bytecode,
			Consts:          consts,
		}
	}

	vm.Blocks = loadedBlocks
	return header.EntryBlockID, nil
}

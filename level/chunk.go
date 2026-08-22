package level

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"strconv"

	"github.com/KonjacBot/go-mc/level/block"
	"github.com/KonjacBot/go-mc/nbt"
	pk "github.com/KonjacBot/go-mc/net/packet"
	"github.com/KonjacBot/go-mc/save"
)

type ChunkPos [2]int32

func (c ChunkPos) WriteTo(w io.Writer) (n int64, err error) {
	n, err = pk.Int(c[0]).WriteTo(w)
	if err != nil {
		return
	}
	n1, err := pk.Int(c[1]).WriteTo(w)
	return n + n1, err
}

func (c *ChunkPos) ReadFrom(r io.Reader) (n int64, err error) {
	var x, z pk.Int
	if n, err = x.ReadFrom(r); err != nil {
		return n, err
	}
	var n1 int64
	if n1, err = z.ReadFrom(r); err != nil {
		return n + n1, err
	}
	*c = ChunkPos{int32(x), int32(z)}
	return n + n1, nil
}

type Chunk struct {
	Sections    []Section
	HeightMaps  HeightMaps
	BlockEntity []BlockEntity
	Light       *LightData
	Status      ChunkStatus
}

func EmptyChunk(secs int) *Chunk {
	sections := make([]Section, secs)
	for i := range sections {
		sections[i] = Section{
			BlockCount: 0,
			States:     NewStatesPaletteContainer(16*16*16, 0),
			Biomes:     NewBiomesPaletteContainer(4*4*4, 0),
		}
	}
	return &Chunk{
		Sections:   sections,
		HeightMaps: HeightMaps{},
		Status:     StatusEmpty,
	}
}

// ChunkFromSave convert save.Chunk to level.Chunk.
func ChunkFromSave(c *save.Chunk) (*Chunk, error) {
	secs := len(c.Sections)
	sections := make([]Section, secs)
	for _, v := range c.Sections {
		i := int32(v.Y) - c.YPos
		if i < 0 || i >= int32(secs) {
			return nil, fmt.Errorf("section Y value %d out of bounds", v.Y)
		}
		var err error
		sections[i].States, err = readStatesPalette(v.BlockStates.Palette, v.BlockStates.Data)
		if err != nil {
			return nil, err
		}
		sections[i].BlockCount = countNoneAirBlocks(&sections[i])
		sections[i].Biomes, err = readBiomesPalette(v.Biomes.Palette, v.Biomes.Data)
		if err != nil {
			return nil, err
		}
		sections[i].SkyLight = v.SkyLight
		sections[i].BlockLight = v.BlockLight
	}

	blockEntities := make([]BlockEntity, len(c.BlockEntities))
	for i, v := range c.BlockEntities {
		var tmp struct {
			ID string `nbt:"id"`
			X  int32  `nbt:"x"`
			Y  int32  `nbt:"y"`
			Z  int32  `nbt:"z"`
		}
		if err := v.Unmarshal(&tmp); err != nil {
			return nil, err
		}
		blockEntities[i].Data = v
		if x, z := int(tmp.X-c.XPos<<4), int(tmp.Z-c.ZPos<<4); !blockEntities[i].PackXZ(x, z) {
			return nil, errors.New("Packing a XZ(" + strconv.Itoa(x) + ", " + strconv.Itoa(z) + ") out of bound")
		}
		blockEntities[i].Y = int16(tmp.Y)
		blockEntities[i].Type = block.EntityTypes[tmp.ID]
	}

	// bitsForHeight := bits.Len( /* chunk height in blocks */ uint(secs)*16 + 1)
	return &Chunk{
		Sections:    sections,
		HeightMaps:  HeightMaps{},
		BlockEntity: blockEntities,
		Status:      ChunkStatus(c.Status),
	}, nil
}

func readStatesPalette(palette []save.BlockState, data []uint64) (paletteData *PaletteContainer[BlocksState], err error) {
	statePalette := make([]BlocksState, len(palette))
	for i, v := range palette {
		b, ok := block.FromID[v.Name]
		if !ok {
			return nil, fmt.Errorf("unknown block id: %v", v.Name)
		}
		if v.Properties.Data != nil {
			if err := v.Properties.Unmarshal(&b); err != nil {
				return nil, fmt.Errorf("unmarshal block properties fail: %v", err)
			}
		}
		s, ok := block.ToStateID[b]
		if !ok {
			return nil, fmt.Errorf("unknown block: %v", b)
		}
		statePalette[i] = s
	}
	paletteData = NewStatesPaletteContainerWithData(16*16*16, data, statePalette)
	return
}

func readBiomesPalette(palette []save.BiomeState, data []uint64) (*PaletteContainer[BiomesState], error) {
	biomesRawPalette := make([]BiomesState, len(palette))
	for i, v := range palette {
		err := biomesRawPalette[i].UnmarshalText([]byte(v))
		if err != nil {
			return nil, err
		}
	}
	return NewBiomesPaletteContainerWithData(4*4*4, data, biomesRawPalette), nil
}

func countNoneAirBlocks(sec *Section) (blockCount int16) {
	for i := 0; i < 16*16*16; i++ {
		b := sec.GetBlock(i)
		if !block.IsAir(b) {
			blockCount++
		}
	}
	return
}

// ChunkToSave convert level.Chunk to save.Chunk
func ChunkToSave(c *Chunk, dst *save.Chunk) (err error) {
	secs := len(c.Sections)
	sections := make([]save.Section, secs)
	for i, v := range c.Sections {
		s := &sections[i]
		states := &s.BlockStates
		biomes := &s.Biomes
		s.Y = int8(int32(i) + dst.YPos)
		states.Palette, states.Data, err = writeStatesPalette(v.States)
		if err != nil {
			return
		}
		biomes.Palette, biomes.Data, err = writeBiomesPalette(v.Biomes)
		if err != nil {
			return
		}
		s.SkyLight = v.SkyLight
		s.BlockLight = v.BlockLight
	}
	dst.Sections = sections
	if dst.Heightmaps == nil {
		dst.Heightmaps = make(map[string][]uint64)
	}
	// dst.Heightmaps["WORLD_SURFACE_WG"] = c.HeightMaps.WorldSurfaceWG.Raw()
	// dst.Heightmaps["WORLD_SURFACE"] = c.HeightMaps.WorldSurface.Raw()
	// dst.Heightmaps["OCEAN_FLOOR_WG"] = c.HeightMaps.OceanFloorWG.Raw()
	// dst.Heightmaps["OCEAN_FLOOR"] = c.HeightMaps.OceanFloor.Raw()
	// dst.Heightmaps["MOTION_BLOCKING"] = c.HeightMaps.MotionBlocking.Raw()
	// dst.Heightmaps["MOTION_BLOCKING_NO_LEAVES"] = c.HeightMaps.MotionBlockingNoLeaves.Raw()
	dst.Status = string(c.Status)
	return
}

func writeStatesPalette(paletteData *PaletteContainer[BlocksState]) (palette []save.BlockState, data []uint64, err error) {
	rawPalette := paletteData.palette.export()
	palette = make([]save.BlockState, len(rawPalette))

	var buffer bytes.Buffer
	for i, v := range rawPalette {
		b := block.StateList[v]
		palette[i].Name = b.ID()

		buffer.Reset()
		err = nbt.NewEncoder(&buffer).Encode(b, "")
		if err != nil {
			return
		}
		_, err = nbt.NewDecoder(&buffer).Decode(&palette[i].Properties)
		if err != nil {
			return
		}
	}

	data = make([]uint64, len(paletteData.data.Raw()))
	copy(data, paletteData.data.Raw())
	return
}

func writeBiomesPalette(paletteData *PaletteContainer[BiomesState]) (palette []save.BiomeState, data []uint64, err error) {
	rawPalette := paletteData.palette.export()
	palette = make([]save.BiomeState, len(rawPalette))

	var biomeID []byte
	for i, v := range rawPalette {
		biomeID, err = v.MarshalText()
		if err != nil {
			return
		}
		palette[i] = save.BiomeState(biomeID)
	}

	data = make([]uint64, len(paletteData.data.Raw()))
	copy(data, paletteData.data.Raw())
	return
}

func (c *Chunk) WriteTo(w io.Writer) (int64, error) {
	data, err := c.Data()
	if err != nil {
		return 0, err
	}
	light := c.Light
	if light == nil {
		derived, err := lightDataFromSections(c.Sections)
		if err != nil {
			return 0, err
		}
		light = &derived
	}
	return pk.Tuple{
		// Heightmaps
		c.HeightMaps,
		pk.ByteArray(data),
		pk.Array(c.BlockEntity),
		light,
	}.WriteTo(w)
}

type HeightMap struct {
	Type int32
	Data []pk.Long
}

func (h *HeightMap) ReadFrom(r io.Reader) (int64, error) {
	var (
		heightmaps struct {
			Type pk.VarInt
			Data []pk.Long
		}
	)
	n, err := pk.Tuple{
		&heightmaps.Type,
		pk.Array(&heightmaps.Data),
	}.ReadFrom(r)
	if err != nil {
		return n, err
	}
	h.Type = int32(heightmaps.Type)
	h.Data = heightmaps.Data
	return n, nil
}

func (h HeightMap) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.VarInt(h.Type),
		pk.Array(h.Data),
	}.WriteTo(w)
}

func (c *Chunk) ReadFrom(r io.Reader) (int64, error) {
	var (
		data pk.ByteArray
	)
	c.Light = new(LightData)

	n, err := pk.Tuple{
		&c.HeightMaps,
		&data,
		pk.Array(&c.BlockEntity),
		c.Light,
	}.ReadFrom(r)
	if err != nil {
		return n, err
	}

	err = c.PutData(data)
	return n, err
}

func (c *Chunk) Data() ([]byte, error) {
	var buff bytes.Buffer
	for i := range c.Sections {
		_, err := c.Sections[i].WriteTo(&buff)
		if err != nil {
			return nil, err
		}
	}
	return buff.Bytes(), nil
}

func (c *Chunk) PutData(data []byte) error {
	r := bytes.NewReader(data)
	for i := range c.Sections {
		_, err := c.Sections[i].ReadFrom(r)
		if errors.Is(err, io.EOF) {
			c.Sections = c.Sections[:i]
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type HeightMaps []HeightMap

func (h *HeightMaps) ReadFrom(r io.Reader) (int64, error) {
	var n int64
	var size pk.VarInt
	nn, err := size.ReadFrom(r)
	n += nn
	if err != nil {
		return n, err
	}
	if size < 0 || size > 64 {
		return n, fmt.Errorf("heightmaps: invalid size %d", size)
	}
	if cap(*h) < int(size) {
		*h = make([]HeightMap, size)
	} else {
		*h = (*h)[:size]
	}
	for i := range *h {
		var typ, dataLen pk.VarInt
		nn, err = typ.ReadFrom(r)
		n += nn
		if err != nil {
			return n, err
		}
		nn, err = dataLen.ReadFrom(r)
		n += nn
		if err != nil {
			return n, err
		}
		if dataLen < 0 {
			return n, fmt.Errorf("heightmaps: invalid data length %d", dataLen)
		}
		(*h)[i].Type = int32(typ)
		(*h)[i].Data = make([]pk.Long, dataLen)
		for j := range (*h)[i].Data {
			nn, err = (*h)[i].Data[j].ReadFrom(r)
			n += nn
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

func (h HeightMaps) WriteTo(w io.Writer) (int64, error) {
	n, err := pk.VarInt(len(h)).WriteTo(w)
	if err != nil {
		return n, err
	}
	for _, heightmap := range h {
		nn, err := pk.VarInt(heightmap.Type).WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
		nn, err = pk.VarInt(len(heightmap.Data)).WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
		for _, v := range heightmap.Data {
			nn, err = v.WriteTo(w)
			n += nn
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

type BlockEntity struct {
	XZ   int8
	Y    int16
	Type block.EntityType
	Data nbt.RawMessage
}

func (b BlockEntity) UnpackXZ() (X, Z int) {
	return int((uint8(b.XZ) >> 4) & 0xF), int(uint8(b.XZ) & 0xF)
}

func (b *BlockEntity) PackXZ(X, Z int) bool {
	if X > 0xF || Z > 0xF || X < 0 || Z < 0 {
		return false
	}
	b.XZ = int8(X<<4 | Z)
	return true
}

func (b BlockEntity) WriteTo(w io.Writer) (n int64, err error) {
	return pk.Tuple{
		pk.Byte(b.XZ),
		pk.Short(b.Y),
		pk.VarInt(b.Type),
		pk.NBT(b.Data),
	}.WriteTo(w)
}

func (b *BlockEntity) ReadFrom(r io.Reader) (n int64, err error) {
	return pk.Tuple{
		(*pk.Byte)(&b.XZ),
		(*pk.Short)(&b.Y),
		(*pk.VarInt)(&b.Type),
		pk.NBT(&b.Data),
	}.ReadFrom(r)
}

type Section struct {
	BlockCount int16
	FluidCount int16
	States     *PaletteContainer[BlocksState]
	Biomes     *PaletteContainer[BiomesState]
	// Half a byte per light value.
	// Could be nil if not exist
	SkyLight   []byte // len() == 2048
	BlockLight []byte // len() == 2048
}

func (s *Section) GetBlock(i int) BlocksState {
	return s.States.Get(i)
}

func (s *Section) SetBlock(i int, v BlocksState) {
	if !block.IsAir(s.States.Get(i)) {
		s.BlockCount--
	}
	if !block.IsAir(v) {
		s.BlockCount++
	}
	s.States.Set(i, v)
}

func (s *Section) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Short(s.BlockCount),
		pk.Short(s.FluidCount),
		s.States,
		s.Biomes,
	}.WriteTo(w)
}

func (s *Section) ReadFrom(r io.Reader) (int64, error) {
	return pk.Tuple{
		(*pk.Short)(&s.BlockCount),
		(*pk.Short)(&s.FluidCount),
		s.States,
		s.Biomes,
	}.ReadFrom(r)
}

type LightData struct {
	SkyLightMask        pk.BitSet
	BlockLightMask      pk.BitSet
	EmptySkyLightMask   pk.BitSet
	EmptyBlockLightMask pk.BitSet
	SkyLight            []pk.ByteArray
	BlockLight          []pk.ByteArray
}

func lightDataFromSections(sections []Section) (LightData, error) {
	lightSectionCount := len(sections) + 2
	wordCount := (lightSectionCount + 63) / 64
	result := LightData{
		SkyLightMask:        make(pk.BitSet, wordCount),
		BlockLightMask:      make(pk.BitSet, wordCount),
		EmptySkyLightMask:   make(pk.BitSet, wordCount),
		EmptyBlockLightMask: make(pk.BitSet, wordCount),
	}

	result.EmptySkyLightMask.Set(0, true)
	result.EmptyBlockLightMask.Set(0, true)
	result.EmptyBlockLightMask.Set(lightSectionCount-1, true)
	hasSkyLight := false
	for index, section := range sections {
		lightIndex := index + 1
		if len(section.SkyLight) == 0 {
			result.EmptySkyLightMask.Set(lightIndex, true)
		} else {
			if len(section.SkyLight) != 2048 {
				return LightData{}, fmt.Errorf(
					"section %d sky light length is %d, want 2048",
					index,
					len(section.SkyLight),
				)
			}
			result.SkyLightMask.Set(lightIndex, true)
			result.SkyLight = append(result.SkyLight, pk.ByteArray(section.SkyLight))
			hasSkyLight = true
		}
		if len(section.BlockLight) == 0 {
			result.EmptyBlockLightMask.Set(lightIndex, true)
		} else {
			if len(section.BlockLight) != 2048 {
				return LightData{}, fmt.Errorf(
					"section %d block light length is %d, want 2048",
					index,
					len(section.BlockLight),
				)
			}
			result.BlockLightMask.Set(lightIndex, true)
			result.BlockLight = append(result.BlockLight, pk.ByteArray(section.BlockLight))
		}
	}
	if hasSkyLight {
		result.SkyLightMask.Set(lightSectionCount-1, true)
		result.SkyLight = append(
			result.SkyLight,
			pk.ByteArray(bytes.Repeat([]byte{0xff}, 2048)),
		)
	} else {
		result.EmptySkyLightMask.Set(lightSectionCount-1, true)
	}
	return result, nil
}

func (l *LightData) WriteTo(w io.Writer) (int64, error) {
	if err := l.validate(); err != nil {
		return 0, err
	}
	return pk.Tuple{
		l.SkyLightMask,
		l.BlockLightMask,
		l.EmptySkyLightMask,
		l.EmptyBlockLightMask,
		pk.Array(l.SkyLight),
		pk.Array(l.BlockLight),
	}.WriteTo(w)
}

func (l *LightData) ReadFrom(r io.Reader) (int64, error) {
	n, err := pk.Tuple{
		&l.SkyLightMask,
		&l.BlockLightMask,
		&l.EmptySkyLightMask,
		&l.EmptyBlockLightMask,
		pk.Array(&l.SkyLight),
		pk.Array(&l.BlockLight),
	}.ReadFrom(r)
	if err != nil {
		return n, err
	}
	return n, l.validate()
}

func (l *LightData) validate() error {
	if bitSetOverlap(l.SkyLightMask, l.EmptySkyLightMask) {
		return errors.New("sky light and empty sky light masks overlap")
	}
	if bitSetOverlap(l.BlockLightMask, l.EmptyBlockLightMask) {
		return errors.New("block light and empty block light masks overlap")
	}
	if got, want := len(l.SkyLight), bitSetOnes(l.SkyLightMask); got != want {
		return fmt.Errorf("sky light array count is %d, mask contains %d sections", got, want)
	}
	if got, want := len(l.BlockLight), bitSetOnes(l.BlockLightMask); got != want {
		return fmt.Errorf("block light array count is %d, mask contains %d sections", got, want)
	}
	for index, data := range l.SkyLight {
		if len(data) != 2048 {
			return fmt.Errorf("sky light array %d length is %d, want 2048", index, len(data))
		}
	}
	for index, data := range l.BlockLight {
		if len(data) != 2048 {
			return fmt.Errorf("block light array %d length is %d, want 2048", index, len(data))
		}
	}
	return nil
}

func bitSetOverlap(a, b pk.BitSet) bool {
	for index := range min(len(a), len(b)) {
		if a[index]&b[index] != 0 {
			return true
		}
	}
	return false
}

func bitSetOnes(set pk.BitSet) int {
	total := 0
	for _, word := range set {
		total += bits.OnesCount64(uint64(word))
	}
	return total
}

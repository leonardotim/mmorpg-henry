package world

type TileType int

const (
	TileGrass TileType = iota
	TileWater
	TileTree
	TileWaterEdgeTop
	TileWaterEdgeBottom
	TileWaterEdgeLeft
	TileWaterEdgeRight
	TileWaterCornerTL
	TileWaterCornerTR
	TileWaterCornerBL
	TileWaterCornerBR
	// New Biomes
	TileWaterDeep
	TileWaterShallow
	TileGrassFlowers
	TileSand
	TileDirtPath
	TileCobblePath
	TileSnow
	TileIce
	TileLava
	TileStoneFloor
	TileWoodFloor
)

func (t TileType) IsSolid() bool {
	switch t {
	case TileWater, TileWaterDeep, TileLava, TileTree, TileWaterCornerBL, TileWaterCornerBR, TileWaterCornerTL, TileWaterCornerTR, TileWaterEdgeBottom, TileWaterEdgeLeft, TileWaterEdgeRight, TileWaterEdgeTop:
		return true
	default:
		return false
	}
}

type Tile struct {
	Type TileType
}

type Map struct {
	Level    int
	Width    int
	Height   int
	Tiles    [][]Tile // Ground Layer
	Objects  [][]int  // Object Layer (0=Empty, >0=ID)
	Spawners []Spawner
}

type Spawner struct {
	X, Y        float64
	CharacterID string
}

func NewMap(width, height int) *Map {
	m := &Map{
		Width:   width,
		Height:  height,
		Tiles:   make([][]Tile, height),
		Objects: make([][]int, height),
	}
	for y := 0; y < height; y++ {
		m.Tiles[y] = make([]Tile, width)
		m.Objects[y] = make([]int, width)
	}
	return m
}

func FlattenTiles(tiles [][]Tile) []int {
	if len(tiles) == 0 {
		return nil
	}
	height := len(tiles)
	width := len(tiles[0])
	flat := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			flat[y*width+x] = int(tiles[y][x].Type)
		}
	}
	return flat
}

func UnflattenTiles(flat []int, width, height int) [][]Tile {
	tiles := make([][]Tile, height)
	for y := 0; y < height; y++ {
		tiles[y] = make([]Tile, width)
		for x := 0; x < width; x++ {
			if y*width+x < len(flat) {
				tiles[y][x] = Tile{Type: TileType(flat[y*width+x])}
			}
		}
	}
	return tiles
}

func FlattenObjects(objects [][]int) []int {
	if len(objects) == 0 {
		return nil
	}
	height := len(objects)
	width := len(objects[0])
	flat := make([]int, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			flat[y*width+x] = objects[y][x]
		}
	}
	return flat
}

func UnflattenObjects(flat []int, width, height int) [][]int {
	objects := make([][]int, height)
	for y := 0; y < height; y++ {
		objects[y] = make([]int, width)
		for x := 0; x < width; x++ {
			if y*width+x < len(flat) {
				objects[y][x] = flat[y*width+x]
			}
		}
	}
	return objects
}

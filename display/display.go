package display

import (
	"fmt"
	"github.com/hajimehoshi/ebiten"
	"github.com/rubinda/GoWorld"
	"image/color"
)

var (
	world GoWorld.World
	// Gender specific colors (for marking dots on the terrain as beings)
	manBlue color.RGBA = color.RGBA{
		R: 103, G: 175, B: 255, A: 255,
	}
	womanViolet color.RGBA = color.RGBA{
		R: 176, G: 101, B: 255, A: 255,
	}
	alienGreen color.RGBA = color.RGBA{
		R: 0, G: 255, B: 0, A: 255,
	}

	beingSprites *BeingSprites
	imgOP        = &ebiten.DrawImageOptions{}
)


// BeingSprite is the image representing a being on the display
type BeingSprite struct {
	Being *GoWorld.Being // The Being this sprite belongs to
	imageWidth  int // Sprite image width
	imageHeight int // Sprite image height
	x           int // Sprite X position on display
	y           int // Sprite Y position on display
	image *ebiten.Image // The sprite image
}

// Update on a being Sprite moves it in the world und updates its coordinates
func (bs *BeingSprite) Update() {
	// Move the being in the terrain package
	world.MoveBeing(bs.Being)

	// Synchronize the positional coordinates with the terrain package
	bs.x = bs.Being.Position.X
	bs.y = bs.Being.Position.Y
}

// BeingSprites is and array of BeingSprite
type BeingSprites struct {
	array []*BeingSprite // The array containing being sprites
	num   int // the length of the array
}

// Update on BeingSprites calls the update function for every individual sprite
func (bss *BeingSprites) Update() {
	for i:=0; i<bss.num; i++ {
		bss.array[i].Update()
	}
}

// BeingSpriteInit initializes the being sprites out of beings already present in the world
func BeingSpriteInit() error {
	// Get the beings from the world and create the BeingSprite array of same size
	beings := world.GetBeings()
	if len(beings) == 0 {
		return fmt.Errorf("error initializing being sprites: no beings to map sprites to")
	}
	beingSprites = &BeingSprites{make([]*BeingSprite, len(beings)), len(beings)}

	// The default color for the being is 'Alien green'
	// This should always change to a specific gender, but just in case ...
	genderColor := alienGreen

	// Initialize the image we will later color as a simple rectangular sprite

	for i := range beings {
		// Check what gender every being is and update the color accordingly
		if beings[i].Gender == "male" {
			genderColor = manBlue
		} else if beings[i].Gender == "female" {
			genderColor = womanViolet
		}
		simpleImg, _ := ebiten.NewImage(10, 10, ebiten.FilterDefault)
		// Paint the simple sprite with the gender based color
		// Fill should return an error but is always null as per documentation (ebiten^1.5)
		_ = simpleImg.Fill(genderColor)
		beingSprites.array[i] = &BeingSprite{
			Being:       beings[i],
			imageWidth:  10,
			imageHeight: 10,
			x:           beings[i].Position.X,
			y:           beings[i].Position.Y,
			image:       simpleImg,
		}

		// Reset the color to alien green
		genderColor = alienGreen
	}
	return nil
}

// update is the ebiten function that handles screen drawing updates
func update(screen *ebiten.Image) error {
	// Draw the background colored terrain (zones)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, 0)
	terrainImage, _ := ebiten.NewImageFromImage(world.GetTerrainImage(), ebiten.FilterDefault)
	screen.DrawImage(terrainImage, op)

	// Update the positions of each being
	beingSprites.Update()

	// Redraw the sprites on screen to match the new positions
	for i:=0; i< beingSprites.num; i++ {
		s := beingSprites.array[i]
		op.GeoM.Reset()
		op.GeoM.Translate(float64(s.x), float64(s.y))
		// As of ebiten 1.5.0 alpha DrawImage() always returns nil, so safe to ignore return value
		_ = screen.DrawImage(s.image, op)
	}
	return nil
}

// Run draws the initial terrain
// Provide screen width and height and a initialized world
func Run(screenWidth, screenHeight int, goworld GoWorld.World) {
	world = goworld // Set the global world variable
	if err := BeingSpriteInit(); err != nil {
		// TODO handle no beings in the world better than panicing
		panic(err)
	}

	// Start the display output
	if err := ebiten.Run(update, screenWidth, screenHeight, 1, "GoWorld"); err != nil {
		panic(err)
	}
}

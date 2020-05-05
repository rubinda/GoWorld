package display

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
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

	beingSprites map[string]*BeingSprite
	foodSprites  map[string]*FoodSprite
)

// BeingSprite is the image representing a being on the display
type BeingSprite struct {
	Being *GoWorld.Being // The Being this sprite belongs to
	x     int            // Sprite X position on display
	y     int            // Sprite Y position on display
	image *ebiten.Image  // The sprite image
}

type FoodSprite struct {
	Food  *GoWorld.Food // The food object the sprite belongs to
	x     int           // Sprite Y position on the display
	y     int           // Sprite X position on the display
	w     int
	h     int
	image *ebiten.Image // The sprite image
}

// Update on a being Sprite moves it in the world und updates its coordinates
func (bs *BeingSprite) Update() {
	// Make the being do an action in the terrain package
	actionDone, uid := world.UpdateBeing(bs.Being)

	// Check if being died => remove it from sprite list
	switch actionDone {
	case "died":
		delete(beingSprites, uid.String())
		return
	case "ate":
		// Remove the food item from screen (being ate it)
		delete(foodSprites, uid.String())

	case "mated":
		// TODO actioneDone == "mated" -> add newly born beings to list

	case "drank":
		// TODO I don't think anything happened with the being?

	}
	// Synchronize the positional coordinates with the terrain package
	bs.x = bs.Being.Position.X
	bs.y = bs.Being.Position.Y
}

// New creates a new food sprite based on ID from GoWorld.Food
func (fs *FoodSprite) New(id uuid.UUID) {
	f := world.GetFoodWithID(id)
	pumpkinImg, _, err := ebitenutil.NewImageFromFile("assets/pumpkin.png", ebiten.FilterDefault)
	if err != nil {
		panic(err)
	}
	foodSprites[id.String()] = &FoodSprite{
		Food:  f,
		x:     f.Position.X,
		y:     f.Position.Y,
		w:     32,
		h:     32,
		image: pumpkinImg,
	}
}

func (fs *FoodSprite) Update() {
	actionDone, uuids := world.UpdatePlant(fs.Food)
	// Check what happened with the plant and update sprites accordingly
	switch actionDone {
	case "withered":
		// The plant died :(
		delete(foodSprites, uuids[0].String())
	case "planted seeds":
		fmt.Println("Drawing plant babies")
		// The plant had babies :)
		for _, uuid := range uuids {
			fs.New(uuid)
		}
	}
}

// BeingSprites is and array of BeingSprite
type BeingSprites struct {
	array []*BeingSprite // The array containing being sprites
	num   int            // the length of the array
}

// FoodSprites is an array of FoodSprite
type FoodSprites struct {
	array []*FoodSprite // Array containing sprites
	num   int           // array length
}

// Update on BeingSprites calls the update function for every individual sprite
func (bss *BeingSprites) Update() {
	for i := 0; i < bss.num; i++ {
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
	beingSprites = make(map[string]*BeingSprite)

	// The default color for the being is 'Alien green'
	// This should always change to a specific gender, but just in case ...
	genderColor := alienGreen

	// Initialize the image we will later color as a simple rectangular sprite
	for _, b := range beings {
		// Check what gender every being is and update the color accordingly
		if b.Gender == "male" {
			genderColor = manBlue
		} else if b.Gender == "female" {
			genderColor = womanViolet
		}
		simpleImg, _ := ebiten.NewImage(10, 10, ebiten.FilterDefault)
		// Paint the simple sprite with the gender based color
		// Fill should return an error but is always null as per documentation (ebiten^1.5)
		_ = simpleImg.Fill(genderColor)
		beingSprites[b.ID.String()] = &BeingSprite{
			Being: b,
			x:     b.Position.X,
			y:     b.Position.Y,
			image: simpleImg,
		}
		// Reset the color to alien green
		genderColor = alienGreen
	}
	return nil
}

func FoodSpriteInit() error {
	food := world.GetFood()
	if len(food) == 0 {
		return fmt.Errorf("error initializing food sprites: no food present")
	}
	foodSprites = make(map[string]*FoodSprite)

	// Initialize a food image (for now all food looks the same)
	pumpkinImg, _, err := ebitenutil.NewImageFromFile("assets/pumpkin.png", ebiten.FilterDefault)
	if err != nil {
		panic(err)
	}
	for _, f := range food {
		foodSprites[f.ID.String()] = &FoodSprite{
			Food:  f,
			x:     f.Position.X,
			y:     f.Position.Y,
			w:     32,
			h:     32,
			image: pumpkinImg,
		}
	}
	return nil
}

// update is the ebiten function that handles screen drawing updates
func update(screen *ebiten.Image) error {
	// Draw the background colored terrain (zones)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, 0)
	terrainImage, _ := ebiten.NewImageFromImage(world.GetTerrainImage(), ebiten.FilterDefault)
	_ = screen.DrawImage(terrainImage, op)

	if ebiten.IsDrawingSkipped() {
		return nil
	}

	// Draw food onto screen
	for _, f := range foodSprites {
		f.Update()
		op.GeoM.Reset()
		op.GeoM.Translate(float64(f.x-f.w/2), float64(f.y-f.h/2))
		_ = screen.DrawImage(f.image, op)
	}

	// Redraw the sprites on screen to match the new positions
	for _, s := range beingSprites {
		s.Update()
		op.GeoM.Reset()
		op.GeoM.Translate(float64(s.x), float64(s.y))
		// As of ebiten 1.5.0 alpha DrawImage() always returns nil, so safe to ignore return value
		_ = screen.DrawImage(s.image, op)
	}

	return nil
}

// Run draws the initial terrain
// Provide screen width and height and a initialized world
func Run(goworld GoWorld.World) {
	world = goworld // Set the global world variable
	if err := BeingSpriteInit(); err != nil {
		// TODO handle no beings in the world better than panicing
		panic(err)
	}
	if err := FoodSpriteInit(); err != nil {
		panic(err)
	}
	screenWidth, screenHeight := world.GetSize()
	// Start the display output
	//ebiten.SetMaxTPS(30)
	if err := ebiten.Run(update, screenWidth, screenHeight, 1, "GoWorld"); err != nil {
		panic(err)
	}
}

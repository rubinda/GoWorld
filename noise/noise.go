// noise represents methods to generate noise, mainly for terrain generation
//
// The (improved) Perlin noise implementation was done with the help of Adrian Biagioli and his well written article @
// https://flafla2.github.io/2014/08/09/perlinnoise.html
//
// The Brownian motion implementation was made using the explanation in this article:
// http://people.bu.edu/andasari/courses/stochasticmodeling/lecture5/stochasticlecture5.html
package noise

import (
	"math"
)

// Perlin represents the Perlin noise generator
type Perlin struct {
	Octaves     float64
	Persistence float64
	p           []int // Used in a hash function to determine which gradient vector to use (quicker than completely random)
}

var (
	Repeat int
	// The predefined permutation table by Ken Perlin in his reference implementation
	// (https://mrl.nyu.edu/~perlin/noise/)
	permutation = [512]int{151, 160, 137, 91, 90, 15, 131, 13, 201, 95, 96, 53, 194, 233, 7, 225, 140, 36, 103,
		30, 69, 142, 8, 99, 37, 240, 21, 10, 23, 190, 6, 148, 247, 120, 234, 75, 0, 26, 197, 62, 94, 252, 219, 203, 117,
		35, 11, 32, 57, 177, 33, 88, 237, 149, 56, 87, 174, 20, 125, 136, 171, 168, 68, 175, 74, 165, 71, 134, 139, 48,
		27, 166, 77, 146, 158, 231, 83, 111, 229, 122, 60, 211, 133, 230, 220, 105, 92, 41, 55, 46, 245, 40, 244, 102,
		143, 54, 65, 25, 63, 161, 1, 216, 80, 73, 209, 76, 132, 187, 208, 89, 18, 169, 200, 196, 135, 130, 116, 188,
		159, 86, 164, 100, 109, 198, 173, 186, 3, 64, 52, 217, 226, 250, 124, 123, 5, 202, 38, 147, 118, 126, 255, 82,
		85, 212, 207, 206, 59, 227, 47, 16, 58, 17, 182, 189, 28, 42, 223, 183, 170, 213, 119, 248, 152, 2, 44, 154,
		163, 70, 221, 153, 101, 155, 167, 43, 172, 9, 129, 22, 39, 253, 19, 98, 108, 110, 79, 113, 224, 232, 178, 185,
		112, 104, 218, 246, 97, 228, 251, 34, 242, 193, 238, 210, 144, 12, 191, 179, 162, 241, 81, 51, 145, 235, 249,
		14, 239, 107, 49, 192, 214, 31, 181, 199, 106, 157, 184, 84, 204, 176, 115, 121, 50, 45, 127, 4, 150, 254, 138,
		236, 205, 93, 222, 114, 67, 29, 24, 72, 243, 141, 128, 195, 78, 66, 215, 61, 156, 180,
	}
)

// NewPerlin sets the Perlin generator attributes to the specified and initializes the permutation table
func NewPerlin(octaves, persistence float64, repeat int) *Perlin {
	p := &Perlin{
		octaves, persistence, make([]int, 512),
	}
	for i := range p.p {
		p.p[i] = permutation[i%256]
	}
	Repeat = repeat
	return p
}

// fade is the fade function used in the improved Perlin noise (6t^5 - 15t^4 + 10t^3)
func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

// lerp represents linear interpolation
// According to the internet "a + w * (b-a)" is the imprecise method due to floating point arithmetic error,
// should be (1-w)*a + w*b
func lerp(a, b, w float64) float64 {
	return (1-w)*a + w*b
}

// grad calculates the dot product between the gradient and distance vectors
//func grad(hash int, x, y, z float64) float64 {
//	h := hash & 15 // Take the hashed value and take the first 4 bits of it (15 == 0b1111)
//	u := y
//	if h < 8 { // If the most significant bit (MSB) of the hash is 0 then set u = x (otherwise leave it at y)
//		u = x
//	}
//	var v float64
//	if h < 4 { // If the first and second significant bits are 0 set v = y
//		v = y
//	} else if h == 12 || h == 14 { // If the first and second significant bits are 1 set v = x
//		v = x
//	} else { // If the first and second significant bits are not equal (0/1, 1/0) set v = z
//		v = z
//	}
//	// Use the last 2 bits to decide if u and v are positive or negative and return their addition
//	if h&1 != 0 {
//		u = -u
//	}
//	if h&2 != 0 {
//		v = -v
//	}
//	return u + v
//}

func grad(hash int, x, y, z float64) float64 {
	switch hash & 0xF {
	case 0x0:
		return x + y
	case 0x1:
		return -x + y
	case 0x2:
		return x - y
	case 0x3:
		return -x - y
	case 0x4:
		return x + z
	case 0x5:
		return -x + z
	case 0x6:
		return x - z
	case 0x7:
		return -x - z
	case 0x8:
		return y + z
	case 0x9:
		return -y + z
	case 0xA:
		return y - z
	case 0xB:
		return -y - z
	case 0xC:
		return y + x
	case 0xD:
		return -y + z
	case 0xE:
		return y - x
	case 0xF:
		return -y - z
	default:
		return 0 // never happens
	}
}

// inc is used to increment the numbers and make sure that the noise repeats if repeat is set
func inc(n int) int {
	n++
	if Repeat > 0 {
		n %= Repeat
	}
	return n
}

// Noise1D returns 1 dimensional noise
func (p *Perlin) Noise1D(x float64) float64 {
	// Todo: rewrite Noise3D to 1D (lesser calculations -> faster)
	return p.Noise3D(x, 0, 0)
}

// Noise2D returns noise for 2 dimensional variables
func (p *Perlin) Noise2D(x, y float64) float64 {
	// Todo: rewrite Noise3D to 2D (lesser calculations -> faster)
	return p.Noise3D(x, y, 0)
}

// OctaveNoise2D return noise combined with different variations of the noise signal
func (p *Perlin) OctaveNoise2D(x, y float64) float64 {
	total := 0.0
	frequency := 1.0
	amplitude := 1.0
	maxValue := 0.0
	// Add up to Octaves different variations of noise and return the sum
	for i:=0.0; i<p.Octaves; i++ {
		total += amplitude * p.Noise2D(x * frequency, y * frequency)

		maxValue += amplitude
		amplitude *= p.Persistence
		frequency *= 2
	}

	return total/maxValue
}

// Noise3D return noise for 3 dimensional variables
func (p *Perlin) Noise3D(x, y, z float64) float64 {
	// Calculate the unit cube around the coordinates
	xi := int(x) & 255
	yi := int(y) & 255
	zi := int(z) & 255

	xf := x - math.Floor(x)
	yf := y - math.Floor(y)
	zf := z - math.Floor(z)

	u := fade(xf)
	v := fade(yf)
	w := fade(zf)

	aaa := p.p[p.p[p.p[xi]+yi]+zi]
	aba := p.p[p.p[p.p[xi]+inc(yi)]+zi]
	aab := p.p[p.p[p.p[xi]+yi]+inc(zi)]
	abb := p.p[p.p[p.p[xi]+inc(yi)]+inc(zi)]
	baa := p.p[p.p[p.p[inc(xi)]+yi]+zi]
	bba := p.p[p.p[p.p[inc(xi)]+inc(yi)]+zi]
	bab := p.p[p.p[p.p[inc(xi)]+yi]+inc(zi)]
	bbb := p.p[p.p[p.p[inc(xi)]+inc(yi)]+inc(zi)]

	var x1, x2, y1, y2 float64
	x1 = lerp(
		grad(aaa, xf, yf, zf),
		grad(baa, xf-1, yf, zf), u)
	x2 = lerp(
		grad(aba, xf, yf-1, zf),
		grad(bba, xf-1, yf-1, zf), u)
	y1 = lerp(x1, x2, v)
	x1 = lerp(
		grad(aab, xf, yf, zf-1),
		grad(bab, xf-1, yf, zf-1), u)
	x2 = lerp(
		grad(abb, xf, yf-1, zf-1),
		grad(bbb, xf-1, yf-1, zf-1), u)
	y2 = lerp(x1, x2, v)

	return (lerp(y1, y2, w) + 1) / 2
}

// OctaveNoise3D returns noise that was combined with different octaves
func (p* Perlin) OctaveNoise3D(x, y, z float64) float64 {
	total := 0.0
	frequency := 1.0
	amplitude := 1.0
	maxValue := 0.0

	for i:=0.0; i<p.Octaves; i++ {
		total += amplitude * p.Noise3D(x * frequency, y * frequency, z * frequency)

		maxValue += amplitude
		amplitude *= p.Persistence
		frequency *= 2
	}

	return total/maxValue
}
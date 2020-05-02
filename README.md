<p align="center"><img src="goworld.png" width="200" alt="GoWorld logo"></p>
<i>Simulate beings walking, eating, drinking and mating on randomly generated terrain. </i>

# GoWorld 
This repository is meant to work as a *simple* simulation of life. Multiple beings can move around the surface and try to survive. It is not a perfect simulation and not many (if any) laws of nature are implemented. Still a fun little project to showcase the capabilities of go.

The terrain generation is done using [Perlin noise](https://flafla2.github.io/2014/08/09/perlinnoise.html), the movement is based on [Brownian motion](https://en.wikipedia.org/wiki/Brownian_motion). Everything else is pretty much randomly generated or chosen.

## Dependencies
+ [ebiten](https://ebiten.org) ... a simple game library

## Installation

#### Requirements:
+ Go 1.12 or later (ebiten requirement)

Use as any other go code:
```sh
go get -u github.com/rubinda/GoWorld
```


## License 

See [LICENSE.md](LICENSE.md)
// The MIT License (MIT)
//
// Copyright (c) 2015-2016 Martin Lindhe
// Copyright (c) 2016      Hajime Hoshi
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package main

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/fogleman/gg"
	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// World represents the game state.
type World struct {
	area   []bool
	width  int
	height int
}

// NewWorld creates a new world.
func NewWorld(width, height int, maxInitLiveCells int) *World {
	w := &World{
		area:   make([]bool, width*height),
		width:  width,
		height: height,
	}
	w.init(maxInitLiveCells)
	return w
}

// init inits world with a random state.
func (w *World) init(maxLiveCells int) {
	for i := 0; i < maxLiveCells; i++ {
		x := rand.Intn(w.width)
		y := rand.Intn(w.height)
		w.area[y*w.width+x] = true
	}
}

// Update game state by one tick.
func (w *World) Update(t *time.Time) {
	width := w.width
	height := w.height
	next := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pop := neighbourCount(w.area, width, height, x, y)
			switch {
			case pop < 2:
				// rule 1. Any live cell with fewer than two live neighbours
				// dies, as if caused by under-population.
				next[y*width+x] = false

			case (pop == 2 || pop == 3) && w.area[y*width+x]:
				// rule 2. Any live cell with two or three live neighbours
				// lives on to the next generation.
				next[y*width+x] = true

			case pop > 3:
				// rule 3. Any live cell with more than three live neighbours
				// dies, as if by over-population.
				next[y*width+x] = false

			case pop == 3:
				// rule 4. Any dead cell with exactly three live neighbours
				// becomes a live cell, as if by reproduction.
				next[y*width+x] = true
			}
		}
	}
	w.area = next
}

// Draw paints current game state.
func (w *World) Draw(pix []byte) {
	for i, v := range w.area {
		if v {
			pix[4*i] = 0xff
			pix[4*i+1] = 0xff
			pix[4*i+2] = 0xff
			pix[4*i+3] = 0xff
		} else {
			pix[4*i] = 0
			pix[4*i+1] = 0
			pix[4*i+2] = 0
			pix[4*i+3] = 0
		}
	}
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// neighbourCount calculates the Moore neighborhood of (x, y).
func neighbourCount(a []bool, width, height, x, y int) int {
	c := 0
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			if i == 0 && j == 0 {
				continue
			}
			x2 := x + i
			y2 := y + j
			if x2 < 0 || y2 < 0 || width <= x2 || height <= y2 {
				continue
			}
			if a[y2*width+x2] {
				c++
			}
		}
	}
	return c
}

const (
	screenWidth  = 640
	screenHeight = 480
)

type Renderer struct {
	world  *World
	pixels []byte
	ch     chan struct{}
	dc     *gg.Context
}

func NewRenderer(world *World, pixels []byte, ch chan struct{}, dc *gg.Context) *Renderer {
	return &Renderer{
		world:  world,
		pixels: pixels,
		ch:     ch,
		dc:     dc,
	}
}

func (r *Renderer) Update() error {
	// r.world.Update() - do nothing!
	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	defer func() {
		defer func() {
			recover()
		}()
		recover()
		r.ch <- struct{}{} // XXX: this is supposed to be invoked when Draw exits!
	}()
	<-r.ch

	// r.dc.DrawCircle(screenWidth/2, screenHeight/2, 20)
	r.dc.SetRGBA(0, 0, 0, 0)
	r.dc.Clear()
	r.dc.SetRGB(1, 1, 1)
	r.dc.SetLineWidth(0.5)
	r.dc.DrawRegularPolygon(6, screenWidth/2, screenHeight/2, 20, 0)
	r.dc.Stroke()

	r.world.Draw(r.pixels)
	screen.ReplacePixels(r.pixels)
	screen.DrawImage(ebiten.NewImageFromImage(r.dc.Image()), nil)
}

func (r *Renderer) Render() {
	defer func() {
		defer func() {
			recover()
		}()
		recover()
		<-r.ch
	}()
	r.ch <- struct{}{}
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func StartRenderingLoop(r *Renderer, ch chan struct{}) {
	go func() {
		runtime.LockOSThread() // XXX: this is required!

		defer func() {
			recover()

			close(r.ch)
			ch <- struct{}{}
		}()

		ebiten.SetWindowSize(screenWidth, screenHeight)
		ebiten.SetWindowTitle("Game of Life (Ebiten Demo)")
		if err := ebiten.RunGame(r); err != nil {
			log.Fatal(err)
		}
	}()
}

func RunWorldUpdateLoop(w *World, r *Renderer, ch chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)
Loop:
	for {
		select {
		case <-ch:
			break Loop
		case t := <-ticker.C:
			fmt.Println("ticker at: ", t)
			w.Update(&t)
			r.Render()
		}
	}
}

func main() {
	w := NewWorld(screenWidth, screenHeight, int((screenWidth*screenHeight)/10))
	r := NewRenderer(w, make([]byte, screenWidth*screenHeight*4), make(chan struct{}), gg.NewContext(screenWidth, screenHeight))

	ch := make(chan struct{})

	StartRenderingLoop(r, ch)
	RunWorldUpdateLoop(w, r, ch)
}
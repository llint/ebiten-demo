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
	"errors"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/SHA65536/Hexago"
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

// Draw renders current world state.
func (w *World) Draw(dc *gg.Context) {
	for i, v := range w.area {
		if v {
			dc.SetPixel(i%w.width, i/w.height)
		}
	}
}

const (
	screenWidth  = 640
	screenHeight = 480
)

type Renderer struct {
	world    *World
	ch       chan struct{}
	dc       *gg.Context
	shutdown atomic.Value
}

func NewRenderer(world *World, dc *gg.Context) *Renderer {
	r := &Renderer{
		world: world,
		ch:    make(chan struct{}),
		dc:    dc,
	}
	r.shutdown.Store(false)
	return r
}

func (r *Renderer) Shutdown() {
	r.shutdown.Store(true)
}

func (r *Renderer) Update() error {
	if r.shutdown.Load().(bool) {
		return errors.New("Shutdown")
	}
	return nil
}

func (r *Renderer) DrawHexagonGrid() {
	grid := Hexago.MakeHexGridWithContext(r.dc, 16, 25)
	grid.SetStrokeAll(0.3, 0.3, 0.3, 1, 1)
	grid.DrawGrid()
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
	// r.dc.SetLineWidth(0.5)
	// r.dc.DrawRegularPolygon(6, screenWidth/2, screenHeight/2, 20, 0)
	// r.dc.Stroke()
	r.DrawHexagonGrid()

	r.dc.SetRGB(1, 1, 1)
	r.world.Draw(r.dc)
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
		ebiten.SetWindowClosingHandled(true)
		if err := ebiten.RunGame(r); err != nil {
			log.Printf("err: %v", err)
		}
	}()
}

func RunWorldUpdateLoop(w *World, r *Renderer, ch chan struct{}) {
	shutdown := time.NewTimer(10 * time.Second)
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
		case <-shutdown.C:
			r.Shutdown()
		}
	}
}

func main() {
	w := NewWorld(screenWidth, screenHeight, int((screenWidth*screenHeight)/10))
	r := NewRenderer(w, gg.NewContext(screenWidth, screenHeight))

	ch := make(chan struct{})

	StartRenderingLoop(r, ch)
	RunWorldUpdateLoop(w, r, ch)
}

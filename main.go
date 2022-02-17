// Simulate lighting in Minecraft.
// The core principle:
// 	Each block is a cell in a cellular system.
//	Each cell has two separate properties:
//		Emission (if it emits light, 0-15) and
//		Light level (this is independent of emission).
//  Here, emission is 0 iff the block at that point does not emit any light.
//  Light level has nothing to do with emission as it could be overwritten by a higher emitting block.
//	Lastly, negative emissions mean light-blocking. In the Java edition of Minecraft, block light is either
//	completely masked out or completely passes through.

package main

import (
	"github.com/gen2brain/raylib-go/raylib"
	"log"
	"strconv"
	"sync"
)

type Point struct {
	X int32
	Y int32
}

func (p Point) neighbors() []Point {
	points := []Point{
		{
			X: p.X - 1,
			Y: p.Y,
		},
		{
			X: p.X + 1,
			Y: p.Y,
		},
		{
			X: p.X,
			Y: p.Y - 1,
		},
		{
			X: p.X,
			Y: p.Y + 1,
		},
	}

	return points
}

// Cell keeps the emission status and light level.
type Cell struct {
	// [0,15] If light source, then >0.
	Source int32

	// [0,15] Light level
	Level int32
}

// Layout is a 16x16 grid of squares
type Layout map[Point]*Cell

const LayoutNSide = 16

func makeEmptyLayout() Layout {
	pattern := Layout{}
	for x := int32(0); x < LayoutNSide; x++ {
		for y := int32(0); y < LayoutNSide; y++ {
			point := Point{X: x, Y: y}
			_, exists := pattern[point]
			if exists {
				continue
			}
			pattern[point] = &Cell{Source: 0, Level: 0}
		}
	}
	return pattern
}

// Calculate the maximum of all neighbors' light levels.
func (layout Layout) maxNeighborsLightLevel(p Point) int32 {
	// You CAN do a proper lock, if you want.
	// My experience is that you don't need to.
	// evolve() is meant to be called many times until it converges (i.e. no more light level changes occur).
	// Regardless of atomicity convergence will be reached.

	max := int32(0)
	for _, neighbor := range p.neighbors() {
		// You can generate (p Point).neighbors() beforehand, and then lock up the affected neighbors, before
		// executing this loop. I don't.
		element, exists := layout[neighbor]
		if !exists {
			continue
		}
		if max < element.Level {
			max = element.Level
		}
	}
	return max
}

func int32Max(a int32, b int32) int32 {
	if a < b {
		return b
	} else {
		return a
	}
}

func cycleLight(level int32) int32 {
	level++
	if level >= 16 {
		level = -1
	}
	return level
}

// Evolve the cellular automata
// Return >0 if it needs to continue.
func (layout *Layout) evolve() int {
	// If the number of blocks that has been altered (i.e. light level changes) is 0
	// then we have reached convergence.
	changed := 0

	// Wait for the pixels (--> voxels) to be processed, since each one will get one logical "thread."
	// The simulation is 2D, hence only 16 * 16 threads.
	joiner := new(sync.WaitGroup)
	joiner.Add(LayoutNSide * LayoutNSide)

	// Data- and flow-independently execute for each block.
	// It doesn't matter if this execution is concurrent or not.
	// Dirty reads are allowed. Convergence will be reached anyway.
	for point, cell := range *layout {
		point := point
		cell := cell

		// Define and then immediately launch a thread for each pixel (--> voxel).
		go func() {
			// Once this thread exits, make sure the joiner/wait group is notified.
			defer joiner.Done()

			// If the emission is negative, then it is a light-blocking block, by our definition.
			if cell.Source < 0 {
				cell.Level = 0
				return
			}

			// Compare the old and new light levels for each pixel.
			// We will record the count changed across all pixels.
			oldLightLevel := cell.Level

			// Determine my (ambient) light level based on my neighbors' (use "largest of n integers" function).
			neighmax := int32Max(layout.maxNeighborsLightLevel(point)-1, 0)
			if cell.Level != neighmax {
				// Note that a cell's light level may increase, stay the same or decrease.
				cell.Level = neighmax
			}

			// If it's light generating, then assume the largest of:
			// - environmental light level and
			// - self-generated light level
			// as its own light level.
			if cell.Source > 0 {
				cell.Level = int32Max(cell.Source, cell.Level)
			}

			if cell.Level != oldLightLevel {
				// If no cells have changed, then, this statement is never executed anyway and "changed" stays at 0.
				// Therefore, no need to do atomic addition. However, it CAN be done to get an accurate report of
				// the number changed.
				changed++
			}
		}()
	}

	joiner.Wait()

	return changed
}

const SquareSideLengthPx = int32(24)

func (layout Layout) raylibDraw() {
	for x := int32(0); x < LayoutNSide; x++ {
		for y := int32(0); y < LayoutNSide; y++ {
			cell, exists := layout[Point{X: x, Y: y}]

			if !exists {
				log.Panicf("Bad layout: values missing at %v", Point{X: x, Y: y})
			}

			// Admittedly the drawing logic isn't really well-thought-out.
			// Rough view.
			//	1. Draw square outline.
			//	2. Draw color inside square.
			//	3. Print number, either ambient light (yellow) or its emission level (orange).

			rl.DrawRectangle(x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, SquareSideLengthPx, rl.Gray)

			var drawColor rl.Color
			if cell.Source > 0 {
				drawColor = rl.ColorAlpha(rl.Orange, float32(cell.Level*LayoutNSide)/256.0)
			} else if cell.Source == 0 {
				drawColor = rl.ColorAlpha(rl.Yellow, float32(cell.Level*LayoutNSide)/256.0)
			}
			rl.DrawRectangle(x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, SquareSideLengthPx, drawColor)

			if cell.Source > 0 {
				rl.DrawText(strconv.Itoa(int(cell.Source)), x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, rl.Black)
			} else if cell.Source == 0 {
				rl.DrawText(strconv.Itoa(int(cell.Level)), x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, rl.Black)
			} else {
				rl.DrawText("x", x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, rl.Black)
			}

			// Draw square boundaries
			rl.DrawRectangleLines(x*SquareSideLengthPx, y*SquareSideLengthPx, SquareSideLengthPx, SquareSideLengthPx, rl.Black)
		}
	}
}

func main() {
	// Test pattern (starter).
	testPattern := makeEmptyLayout()
	testPattern[Point{X: 1, Y: 1}] = &Cell{Source: 15, Level: 0}

	// 64 px --- give it some space at the bottom for extra text
	rl.InitWindow(LayoutNSide*SquareSideLengthPx, LayoutNSide*SquareSideLengthPx+64, "Minecraft lighting automata demo (pixels)")

	// 10 fps is fast enough
	rl.SetTargetFPS(10)

	for !rl.WindowShouldClose() {
		// Update

		// Mouse ... Pressed = only once
		// Mouse ... Down = as long as pressed

		if rl.IsMouseButtonPressed(rl.MouseRightButton) {
			// Right click to reset cell

			// Poll the cell location
			guessX := rl.GetMouseX() / SquareSideLengthPx
			guessY := rl.GetMouseY() / SquareSideLengthPx

			// In range? Do it.
			if 0 <= guessX && guessX < LayoutNSide &&
				0 <= guessY && guessY < LayoutNSide {
				// Cycle the light level.
				cell, exists := testPattern[Point{X: guessX, Y: guessY}]

				if !exists {
					// No big deal if the guess fails. Just note it and then move on.
					log.Printf("Guess failed mouse X, Y = (%d, %d) ==> gX, gY = (%d, %d)\n",
						rl.GetMouseX(), rl.GetMouseY(), guessX, guessY)
				}

				if cell.Source == -1 {
					cell.Source = 0
				} else {
					cell.Source = -1
				}
			}
		}

		if rl.IsMouseButtonDown(rl.MouseLeftButton) {
			// Poll the cell location
			guessX := rl.GetMouseX() / SquareSideLengthPx
			guessY := rl.GetMouseY() / SquareSideLengthPx

			// In range? Do it.
			if 0 <= guessX && guessX < LayoutNSide &&
				0 <= guessY && guessY < LayoutNSide {
				// Cycle the light level.
				cell, exists := testPattern[Point{X: guessX, Y: guessY}]

				if !exists {
					// No big deal if the guess fails. Just note it and then move on.
					log.Printf("Guess failed mouse X, Y = (%d, %d) ==> gX, gY = (%d, %d)\n",
						rl.GetMouseX(), rl.GetMouseY(), guessX, guessY)
				}

				newSource := cycleLight(cell.Source)

				testPattern[Point{X: guessX, Y: guessY}].Source = newSource
			}
		}

		if rl.IsKeyPressed(rl.KeyR) {
			// Reset everything
			testPattern = makeEmptyLayout()
		}

		// Drawing
		rl.BeginDrawing()

		rl.ClearBackground(rl.RayWhite)

		testPattern.raylibDraw()

		rl.DrawText("left-clk: increase; right: clear\n<R>: reset; credit @0wulfaz", 0, LayoutNSide*SquareSideLengthPx, 24, rl.Black)

		changed := testPattern.evolve()
		log.Printf("Number changed: %v\n", changed)

		rl.EndDrawing()
	}

	rl.CloseWindow()
}

// TideBreaker — a nostalgic single-player breakout clone built with Fyne.
//
// Steer the paddle with the mouse or the arrow keys (A/D also work), keep the
// ball in play, and break as many ocean-blue blocks as you can. Clearing the
// board advances you to a faster level; the game ends when your lives run out.
//
// Controls:
//
//	Mouse / ← → / A D  move the paddle
//	Space              launch the ball · pause/resume · play again
//	P                  pause / resume
//	R                  restart
//
// Play also pauses automatically whenever the app leaves the foreground.
package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

const targetFPS = 60

func main() {
	a := app.New()
	w := a.NewWindow("TideBreaker")

	game := NewGame()
	game.onSound = newSoundPlayer().Play
	board := NewBoard(game)
	w.SetContent(container.NewStack(board))
	w.Resize(fyne.NewSize(640, 600))

	// Give the board keyboard focus so held arrow keys reach it.
	w.Canvas().Focus(board)

	a.Lifecycle().SetOnExitedForeground(func() {
		game.Pause()
		board.Refresh()
	})

	startLoop(game, board)
	w.ShowAndRun()
}

// startLoop drives the simulation on a fixed-rate ticker. All state changes and
// the canvas refresh are marshalled onto the UI goroutine via fyne.Do, so the
// model is only ever touched from one thread and needs no locking.
func startLoop(game *Game, board *Board) {
	go func() {
		ticker := time.NewTicker(time.Second / targetFPS)
		defer ticker.Stop()
		last := time.Now()
		for now := range ticker.C {
			dt := float32(now.Sub(last).Seconds())
			last = now
			if dt > 0.05 { // clamp after a stall so the ball never teleports
				dt = 0.05
			}
			fyne.Do(func() {
				// Do the least work the frame needs: nothing when idle, a cheap
				// ball/paddle reposition while the ball flies, and a full redraw
				// only when something structural (bricks, score, banner) changed.
				switch game.Tick(dt) {
				case RenderMove:
					board.MoveDynamic()
				case RenderFull:
					board.Refresh()
				}
			})
		}
	}()
}

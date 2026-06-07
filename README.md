# TideBreaker

A nostalgic, single-player **breakout** clone built with [Fyne](https://fyne.io).
Steer your paddle, keep the ball in play, and break as many ocean-blue blocks as
you can. Clear the board and the tide rises — the next level launches faster.
The game ends when your lives run out.

![TideBreaker](https://img.shields.io/badge/Fyne-v2.7-blue)

## Play

```sh
go run .
```

To build a standalone binary:

```sh
go build -o tidebreaker .
./tidebreaker
```

## Controls

| Input | Action |
| --- | --- |
| Mouse move / `←` `→` / `A` `D` | Move the paddle |
| `Space` | Launch the ball · pause / resume · play again |
| `P` | Pause / resume |
| `R` | Restart |

## How it plays

- **Three lives.** Lose one whenever the ball slips past your paddle. At zero, it's game over.
- **Score by row.** Higher rows are worth more (60 at the top down to 10 at the bottom).
- **Paddle english.** Where the ball hits the paddle steers its bounce — catch it
  near the edge to angle a shot, hit it dead-centre to send it straight back up.
- **Rising tide.** Each broken brick nudges the ball a little faster, and every
  cleared board starts the next level quicker still. Survive as long as you can.

- **Retro sound.** Every bounce, brick, lost life and game over plays a synthesized
  square-wave blip — no audio assets, just tones generated in code. If no audio
  device is available the game runs silently with no error.

The window is fully resizable; the whole board scales to fit.

## Project layout

| File | Responsibility |
| --- | --- |
| `game.go` | Pure game state and physics — collisions, scoring, levels (no GUI or audio) |
| `board.go` | The Fyne custom widget: rendering and mouse/keyboard input |
| `sound.go` | Synthesizes the retro blips and plays them via `oto` |
| `main.go` | App setup and the fixed-rate game loop |
| `game_test.go` | Headless tests for the physics and rules |

Because the model in `game.go` has no Fyne dependency, the rules are tested
without a display:

```sh
go test ./...
```

## Implementation notes

- Rendering uses Fyne `canvas` primitives (`Rectangle`, `Circle`, `Text`) driven
  by a custom `WidgetRenderer`, rather than per-pixel rasterising.
- A 60 FPS ticker advances the simulation. All state changes and the canvas
  refresh are marshalled onto the UI goroutine via `fyne.Do`, so the model is
  only ever touched from one thread and needs no locking.
- Brick collisions use a Minkowski-inset test (the brick expanded by the ball
  radius), bouncing off whichever edge the ball penetrated least.
- Sound is decoupled from the model: `game.go` only emits `Sound` events through a
  callback, so the rules stay pure and the tests run silently. `sound.go` renders
  each effect once as 16-bit PCM (square waves with a short attack/release
  envelope) and plays them through [`oto`](https://github.com/ebitengine/oto),
  which needs no CGO and no audio asset files.

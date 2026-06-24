package main

import (
	"image/color"
	"math"
	"math/rand"
)

// GameState models the high-level phase of play.
type GameState int

const (
	StateReady    GameState = iota // ball resting on the paddle, waiting to launch
	StatePlaying                   // ball in motion
	StatePaused                    // frozen by the player
	StateGameOver                  // out of lives
	StateWon                       // every level cleared
)

const (
	startLives = 3
	brickCols  = 10
	brickRows  = 6           // rows in the opening (classic full-grid) level
	maxBounce  = math.Pi / 3 // 60° — steepest deflection off the paddle edge
)

// maxLevel is the number of hand-designed boards; clearing the last one wins.
var maxLevel = len(levels)

// Sound names an audible game event. The model only emits these through a
// callback, so it stays free of any audio dependency (and the tests stay silent).
type Sound int

const (
	SoundLaunch Sound = iota
	SoundPaddle
	SoundWall
	SoundBrick
	SoundLoseLife
	SoundGameOver
	SoundLevelUp
	SoundWin
)

// Brick is a single block. Most bricks shatter in one hit; tougher bricks take
// several, and unbreakable bricks only ever deflect the ball.
type Brick struct {
	x, y, w, h  float32
	col         color.Color
	points      int
	hits        int  // blows still needed to destroy it (ignored when unbreakable)
	maxHits     int  // hits when fresh, used to pick the damage colour
	unbreakable bool // a fixed wall that never breaks and never blocks level clear
	alive       bool
}

// Game holds all mutable play state. Every field is touched only from the UI
// goroutine (the ticker schedules updates via fyne.Do), so no locking is needed.
type Game struct {
	w, h        float32 // logical board size, kept equal to the widget size
	initialized bool

	paddleX, paddleW, paddleH, paddleY float32
	paddleSpeed                        float32 // keyboard glide speed, px/s
	leftHeld, rightHeld                bool

	ballX, ballY   float32
	ballVX, ballVY float32
	ballR          float32
	speed          float32 // current ball speed magnitude, px/s
	baseSpeed      float32 // speed the ball launches at this level

	bricks []*Brick

	score int
	lives int
	level int
	state GameState

	// render records how much redrawing the next loop iteration needs. The loop
	// does nothing on RenderNone (idle game), only repositions the ball/paddle on
	// RenderMove (the common case — the ball flying), and re-runs the full
	// renderer on RenderFull (bricks, score, banner — anything structural).
	render Render

	onSound func(Sound) // optional sink for sound events; nil = silent
}

// Render is how much work a frame needs: none, a cheap reposition, or a full redraw.
type Render int

const (
	RenderNone Render = iota // nothing changed; skip the frame entirely
	RenderMove               // only the ball/paddle moved; just reposition them
	RenderFull               // structural change; re-run the whole renderer
)

// needRender escalates the pending work for this frame to at least level.
func (g *Game) needRender(level Render) {
	if level > g.render {
		g.render = level
	}
}

// play emits a sound event if a sink is wired up.
func (g *Game) play(s Sound) {
	if g.onSound != nil {
		g.onSound(s)
	}
}

// rowColors paints an ocean gradient from foam at the top to deep water below.
var rowColors = []color.Color{
	color.NRGBA{0xA7, 0xF3, 0xD0, 0xFF}, // foam
	color.NRGBA{0x5E, 0xEA, 0xD4, 0xFF}, // aqua
	color.NRGBA{0x22, 0xD3, 0xEE, 0xFF}, // cyan
	color.NRGBA{0x38, 0xBD, 0xF8, 0xFF}, // sky
	color.NRGBA{0x3B, 0x82, 0xF6, 0xFF}, // blue
	color.NRGBA{0x63, 0x66, 0xF1, 0xFF}, // deep
}

// rowPoints rewards the harder-to-reach upper rows more generously.
var rowPoints = []int{60, 50, 40, 30, 20, 10}

// Special brick colours. Tougher bricks pale as they take damage so the player
// can read how close they are to breaking.
var (
	colUnbreak = color.NRGBA{0x47, 0x55, 0x69, 0xFF} // slate — a permanent wall
	silverCols = []color.Color{                      // index by remaining hits - 1
		color.NRGBA{0x94, 0xA3, 0xB8, 0xFF}, // 1 left — cracked
		color.NRGBA{0xCB, 0xD5, 0xE1, 0xFF}, // 2 left — bright silver
	}
	goldCols = []color.Color{
		color.NRGBA{0xB9, 0x8A, 0x24, 0xFF}, // 1 left
		color.NRGBA{0xE3, 0xB3, 0x3A, 0xFF}, // 2 left
		color.NRGBA{0xFD, 0xE0, 0x68, 0xFF}, // 3 left — fresh gold
	}
)

// Special brick scores. Tougher bricks are worth more.
const (
	silverPoints = 50
	goldPoints   = 90
)

// levels holds the 10 hand-designed boards, easiest first. Each string is one
// row of up to brickCols cells:
//
//	. or space  empty
//	o           standard brick (one hit, ocean-gradient colour by row)
//	2           silver brick   (two hits)
//	3           gold brick     (three hits)
//	#           unbreakable wall (deflects only; never blocks level clear)
//
// Patterns avoid sealing breakable bricks behind walls, so every board is
// always clearable.
var levels = [][]string{
	// 1 — Classic warm-up: a solid wall of single-hit bricks.
	{
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
	},
	// 2 — Denser wall, two rows taller.
	{
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
		"oooooooooo",
	},
	// 3 — Checkerboard: gaps let the ball weave through.
	{
		"o.o.o.o.o.",
		".o.o.o.o.o",
		"o.o.o.o.o.",
		".o.o.o.o.o",
		"o.o.o.o.o.",
		".o.o.o.o.o",
	},
	// 4 — Pyramid with a silver-tipped peak.
	{
		"....22....",
		"...o22o...",
		"..oooooo..",
		".oooooooo.",
		"oooooooooo",
	},
	// 5 — Vertical pillars guarding a silver core.
	{
		"oo..oo..oo",
		"oo..22..oo",
		"oo..22..oo",
		"oo..22..oo",
		"oo..oo..oo",
		"oo..oo..oo",
	},
	// 6 — Diamond ringed with silver.
	{
		"....oo....",
		"...o22o...",
		"..oo22oo..",
		".oo2oo2oo.",
		"..oo22oo..",
		"...o22o...",
		"....oo....",
	},
	// 7 — Descending stripes, the leading edge reinforced.
	{
		"222.......",
		".ooo......",
		"..222.....",
		"...ooo....",
		"....222...",
		".....ooo..",
		"......222.",
		".......ooo",
	},
	// 8 — Fortress: unbreakable bunkers embedded in the wall.
	{
		"oooooooooo",
		"o##oooo##o",
		"oooooooooo",
		"o##oooo##o",
		"oooooooooo",
		"oooo22oooo",
	},
	// 9 — Tough wall: gold and silver up top, easing toward the paddle.
	{
		"3333333333",
		"2222222222",
		"3333333333",
		"2222222222",
		"oooooooooo",
		"oooooooooo",
	},
	// 10 — The gauntlet: a gold/silver lattice over a guarded base.
	{
		"3232323232",
		"2323232323",
		"3232323232",
		"2323232323",
		"oooooooooo",
		"#oooooooo#",
	},
}

// multiHitColor returns the colour for a tough brick with `remaining` hits left.
func multiHitColor(maxHits, remaining int) color.Color {
	var pal []color.Color
	switch maxHits {
	case 2:
		pal = silverCols
	case 3:
		pal = goldCols
	default:
		return colUnbreak
	}
	i := clampInt(remaining-1, 0, len(pal)-1)
	return pal[i]
}

// makeBrick builds a brick from a pattern cell. row picks the gradient colour
// and point value for standard bricks.
func makeBrick(ch byte, row int) *Brick {
	switch ch {
	case '2':
		return &Brick{col: multiHitColor(2, 2), points: silverPoints, hits: 2, maxHits: 2, alive: true}
	case '3':
		return &Brick{col: multiHitColor(3, 3), points: goldPoints, hits: 3, maxHits: 3, alive: true}
	case '#':
		return &Brick{col: colUnbreak, unbreakable: true, alive: true}
	default: // 'o'
		return &Brick{
			col:     rowColors[row%len(rowColors)],
			points:  rowPoints[row%len(rowPoints)],
			hits:    1,
			maxHits: 1,
			alive:   true,
		}
	}
}

func NewGame() *Game {
	return &Game{
		w:      640,
		h:      800,
		lives:  startLives,
		level:  1,
		state:  StateReady,
		render: RenderFull, // force the opening frame to draw
	}
}

// Resize keeps the logical board matched to the widget. The first call lays out
// the opening level; later calls rescale everything so a window resize is smooth.
func (g *Game) Resize(w, h float32) {
	if w <= 0 || h <= 0 {
		return
	}
	g.needRender(RenderFull) // geometry changed: everything must be repositioned
	if !g.initialized {
		g.w, g.h = w, h
		g.initialized = true
		g.startGame()
		return
	}
	sx, sy := w/g.w, h/g.h
	g.w, g.h = w, h

	g.paddleW *= sx
	g.paddleH *= sy
	g.paddleX *= sx
	g.paddleY *= sy
	g.paddleSpeed *= sx
	g.ballX *= sx
	g.ballY *= sy
	g.ballVX *= sx
	g.ballVY *= sy

	// scale ball radius and speed to the actual size of our screen.
	g.ballR *= float32(math.Sqrt(float64(sx * sy)))
	g.speed *= sy
	g.baseSpeed *= sy
	for _, b := range g.bricks {
		b.x *= sx
		b.y *= sy
		b.w *= sx
		b.h *= sy
	}
}

// startGame resets score and lives and builds the first level.
func (g *Game) startGame() {
	g.score = 0
	g.lives = startLives
	g.level = 1
	g.setupLevel()
}

// setupLevel builds the brick grid for g.level and parks the ball on the paddle.
func (g *Game) setupLevel() {
	g.paddleW = g.w * 0.16
	g.paddleH = g.h * 0.02
	g.paddleY = g.h - g.paddleH*3
	g.paddleX = (g.w - g.paddleW) / 2
	g.paddleSpeed = g.w * 1.5
	g.ballR = g.h * 0.011

	// Each cleared level launches a little faster, capped so it stays playable.
	g.baseSpeed = g.h * 0.62 * float32(math.Pow(1.06, float64(g.level-1)))
	if max := g.h * 1.15; g.baseSpeed > max {
		g.baseSpeed = max
	}
	g.speed = g.baseSpeed

	side := g.w * 0.04
	top := g.h * 0.12
	gap := g.w * 0.008
	areaW := g.w - 2*side
	bw := (areaW - gap*float32(brickCols-1)) / brickCols
	bh := g.h * 0.028

	// Levels are 1-indexed; wrap defensively so an out-of-range level can never
	// panic (play stops at StateWon before this happens in normal flow).
	pattern := levels[(g.level-1)%len(levels)]

	g.bricks = g.bricks[:0]
	for row, line := range pattern {
		for col := 0; col < brickCols && col < len(line); col++ {
			ch := line[col]
			if ch == '.' || ch == ' ' {
				continue
			}
			b := makeBrick(ch, row)
			b.x = side + float32(col)*(bw+gap)
			b.y = top + float32(row)*(bh+gap)
			b.w, b.h = bw, bh
			g.bricks = append(g.bricks, b)
		}
	}
	g.resetBall()
}

// resetBall parks the ball on the paddle and waits for a launch.
func (g *Game) resetBall() {
	g.speed = g.baseSpeed
	g.state = StateReady
	g.stickBall()
	g.needRender(RenderFull)
}

// stickBall glues the ball to the centre-top of the paddle (used while Ready).
func (g *Game) stickBall() {
	g.ballX = g.paddleX + g.paddleW/2
	g.ballY = g.paddleY - g.ballR - 1
	g.ballVX, g.ballVY = 0, 0
}

// Launch fires the ball upward at a slight random angle.
func (g *Game) Launch() {
	if g.state != StateReady {
		return
	}
	angle := (rand.Float64()*2 - 1) * float64(maxBounce) * 0.35
	g.ballVX = g.speed * float32(math.Sin(angle))
	g.ballVY = -g.speed * float32(math.Cos(angle))
	g.state = StatePlaying
	g.needRender(RenderFull)
	g.play(SoundLaunch)
}

// TogglePause flips between Playing and Paused (no-op in other states).
func (g *Game) TogglePause() {
	switch g.state {
	case StatePlaying:
		g.state = StatePaused
		g.needRender(RenderFull)
	case StatePaused:
		g.state = StatePlaying
		g.needRender(RenderFull)
	}
}

// Pause freezes play if the ball is in motion. Unlike TogglePause it only ever
// pauses, never resumes.
func (g *Game) Pause() {
	if g.state == StatePlaying {
		g.state = StatePaused
		g.needRender(RenderFull)
	}
}

// Restart begins a brand new game from the current board size.
func (g *Game) Restart() {
	g.startGame()
}

// SetPaddleCenter aims the paddle so its centre sits under x (mouse control).
func (g *Game) SetPaddleCenter(x float32) {
	if g.state == StateGameOver || g.state == StatePaused || g.state == StateWon {
		return
	}
	nx := clamp(x-g.paddleW/2, 0, g.w-g.paddleW)
	if nx != g.paddleX {
		g.paddleX = nx
		g.needRender(RenderMove)
	}
}

// Tick advances the game by dt seconds and reports how much redrawing the frame
// needs. It returns RenderNone for an idle frame (nothing moving, no pending
// input), RenderMove when only the ball/paddle shifted, and RenderFull after a
// structural change, so the loop can do the least work possible.
func (g *Game) Tick(dt float32) Render {
	switch g.state {
	case StatePlaying:
		g.movePaddle(dt)
		g.moveBall(dt)           // collisions escalate to RenderFull as needed
		g.needRender(RenderMove) // the ball is always in motion while playing
	case StateReady:
		g.movePaddle(dt) // escalates to RenderMove only if the paddle glides
		g.stickBall()    // the parked ball just tracks the paddle
	}
	r := g.render
	g.render = RenderNone
	return r
}

// movePaddle applies held-key gliding and keeps the paddle on-board.
func (g *Game) movePaddle(dt float32) {
	prev := g.paddleX
	if g.leftHeld {
		g.paddleX -= g.paddleSpeed * dt
	}
	if g.rightHeld {
		g.paddleX += g.paddleSpeed * dt
	}
	g.paddleX = clamp(g.paddleX, 0, g.w-g.paddleW)
	if g.paddleX != prev {
		g.needRender(RenderMove)
	}
}

// moveBall integrates the ball and resolves wall, paddle and brick collisions.
func (g *Game) moveBall(dt float32) {
	g.ballX += g.ballVX * dt
	g.ballY += g.ballVY * dt
	r := g.ballR

	// Side and top walls.
	if g.ballX-r < 0 {
		g.ballX = r
		g.ballVX = abs(g.ballVX)
		g.play(SoundWall)
	}
	if g.ballX+r > g.w {
		g.ballX = g.w - r
		g.ballVX = -abs(g.ballVX)
		g.play(SoundWall)
	}
	if g.ballY-r < 0 {
		g.ballY = r
		g.ballVY = abs(g.ballVY)
		g.play(SoundWall)
	}

	// Bottom — the ball is lost.
	if g.ballY-r > g.h {
		g.loseLife()
		return
	}

	g.paddleCollision()
	g.brickCollisions()

	if g.breakableAlive() == 0 {
		g.advanceLevel()
	}
}

// advanceLevel loads the next board, or declares victory after the final one.
func (g *Game) advanceLevel() {
	if g.level >= maxLevel {
		g.state = StateWon
		g.needRender(RenderFull)
		g.play(SoundWin)
		return
	}
	g.level++
	g.setupLevel()
	g.play(SoundLevelUp)
}

// paddleCollision bounces the ball off the paddle, steering it by where it lands.
func (g *Game) paddleCollision() {
	if g.ballVY <= 0 {
		return
	}
	r := g.ballR
	if g.ballY+r < g.paddleY || g.ballY-r > g.paddleY+g.paddleH {
		return
	}
	if g.ballX+r < g.paddleX || g.ballX-r > g.paddleX+g.paddleW {
		return
	}
	g.ballY = g.paddleY - r
	rel := clamp((g.ballX-(g.paddleX+g.paddleW/2))/(g.paddleW/2), -1, 1)
	angle := float64(rel) * float64(maxBounce)
	g.ballVX = g.speed * float32(math.Sin(angle))
	g.ballVY = -g.speed * float32(math.Cos(angle))
	g.play(SoundPaddle)
}

// brickCollisions resolves at most one brick per frame using Minkowski insets.
func (g *Game) brickCollisions() {
	r := g.ballR
	for _, b := range g.bricks {
		if !b.alive {
			continue
		}
		// Expand the brick by the ball radius and test the ball centre.
		ex0, ey0 := b.x-r, b.y-r
		ex1, ey1 := b.x+b.w+r, b.y+b.h+r
		if g.ballX < ex0 || g.ballX > ex1 || g.ballY < ey0 || g.ballY > ey1 {
			continue
		}
		// Choose the axis of shallowest penetration to bounce off.
		left := g.ballX - ex0
		right := ex1 - g.ballX
		top := g.ballY - ey0
		bottom := ey1 - g.ballY
		m := min4(left, right, top, bottom)
		switch m {
		case left:
			g.ballX, g.ballVX = ex0, -abs(g.ballVX)
		case right:
			g.ballX, g.ballVX = ex1, abs(g.ballVX)
		case top:
			g.ballY, g.ballVY = ey0, -abs(g.ballVY)
		default:
			g.ballY, g.ballVY = ey1, abs(g.ballVY)
		}

		// Unbreakable walls only deflect — they never break or score.
		if b.unbreakable {
			g.play(SoundWall)
			return
		}

		// Tough bricks survive a few blows, paling as they crack.
		b.hits--
		if b.hits > 0 {
			b.col = multiHitColor(b.maxHits, b.hits)
			g.needRender(RenderFull) // the brick's colour changed
			g.play(SoundWall)
			return
		}

		b.alive = false
		g.score += b.points
		g.needRender(RenderFull) // a brick vanished and the score moved
		g.play(SoundBrick)
		// Each broken brick nudges the pace up a touch.
		g.speed *= 1.004
		g.rescaleVelocity()
		return
	}
}

// rescaleVelocity keeps the velocity vector pointed the same way at g.speed.
func (g *Game) rescaleVelocity() {
	mag := float32(math.Hypot(float64(g.ballVX), float64(g.ballVY)))
	if mag == 0 {
		return
	}
	s := g.speed / mag
	g.ballVX *= s
	g.ballVY *= s
}

func (g *Game) loseLife() {
	g.lives--
	if g.lives <= 0 {
		g.lives = 0
		g.state = StateGameOver
		g.needRender(RenderFull) // show the game-over banner
		g.play(SoundGameOver)
		return
	}
	g.play(SoundLoseLife)
	g.resetBall() // resets to Ready and marks RenderFull
}

func (g *Game) aliveBricks() int {
	n := 0
	for _, b := range g.bricks {
		if b.alive {
			n++
		}
	}
	return n
}

// breakableAlive counts the bricks still standing that the player can destroy;
// unbreakable walls are ignored so they never stall level completion.
func (g *Game) breakableAlive() int {
	n := 0
	for _, b := range g.bricks {
		if b.alive && !b.unbreakable {
			n++
		}
	}
	return n
}

// --- small float helpers ---

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func min4(a, b, c, d float32) float32 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	if d < m {
		m = d
	}
	return m
}

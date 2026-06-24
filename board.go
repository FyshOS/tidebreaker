package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Theme colours for the non-brick elements.
var (
	colWater  = color.NRGBA{0x0B, 0x1B, 0x2B, 0xFF} // deep navy background
	colPaddle = color.NRGBA{0xFD, 0xE6, 0x8A, 0xFF} // warm sand
	colBall   = color.NRGBA{0xF8, 0xFA, 0xFC, 0xFF} // foam white
	colText   = color.NRGBA{0xE2, 0xE8, 0xF0, 0xFF}
	colDim    = color.NRGBA{0x94, 0xA3, 0xB8, 0xFF}
	colOver   = color.NRGBA{0x0B, 0x1B, 0x2B, 0xC8} // translucent veil for pause/over screens
)

// Board is the interactive play surface: a focusable, hoverable custom widget
// that renders the game and feeds player input into the model.
type Board struct {
	widget.BaseWidget
	game     *Game
	renderer *boardRenderer // cached so the loop can reposition without a full Refresh
}

func NewBoard(g *Game) *Board {
	b := &Board{game: g}
	b.ExtendBaseWidget(b)
	return b
}

// CreateRenderer wires up the persistent canvas objects.
func (b *Board) CreateRenderer() fyne.WidgetRenderer {
	r := &boardRenderer{board: b, game: b.game}

	r.bg = canvas.NewRectangle(colWater)
	r.over = canvas.NewRectangle(colOver)
	r.over.Hide()
	r.paddle = canvas.NewRectangle(colPaddle)
	r.paddle.CornerRadius = 6
	r.ball = canvas.NewCircle(colBall)

	r.score = canvas.NewText("", colText)
	r.score.TextStyle.Bold = true
	r.lives = canvas.NewText("", colText)
	r.lives.TextStyle.Bold = true
	r.lives.Alignment = fyne.TextAlignTrailing

	r.levelText = canvas.NewText("", colDim)
	r.levelText.TextStyle.Bold = true
	r.levelText.Alignment = fyne.TextAlignCenter

	r.title = canvas.NewText("", colText)
	r.title.TextStyle.Bold = true
	r.title.Alignment = fyne.TextAlignCenter
	r.subtitle = canvas.NewText("", colDim)
	r.subtitle.Alignment = fyne.TextAlignCenter

	b.renderer = r
	return r
}

// MoveDynamic repositions only the ball and paddle, the two objects that move
// during normal play, without re-running the full renderer. It triggers a single
// lightweight canvas repaint and skips all the HUD/brick/banner work.
func (b *Board) MoveDynamic() {
	if b.renderer != nil {
		b.renderer.moveDynamic()
	}
}

// --- input: pointer (mouse hover, click, and touch) ---

func (b *Board) MouseIn(*desktop.MouseEvent) {}
func (b *Board) MouseOut()                   {}

// MouseMoved steers the paddle as the mouse hovers across the board (desktop).
func (b *Board) MouseMoved(e *desktop.MouseEvent) {
	b.game.SetPaddleCenter(e.Position.X)
}

// Tapped runs the same action as the Space key.
func (b *Board) Tapped(*fyne.PointEvent) {
	b.activate()
}

// Dragged steers the paddle to follow a finger (or a dragged mouse) across the
// board to move paddle.
func (b *Board) Dragged(e *fyne.DragEvent) {
	b.game.SetPaddleCenter(e.Position.X)
}

func (b *Board) DragEnd() {}

// activate runs the primary action: launch a parked ball, toggle pause during
// play, or start a fresh game once it's over.
func (b *Board) activate() {
	switch b.game.state {
	case StateReady:
		b.game.Launch()
	case StatePlaying, StatePaused:
		b.game.TogglePause()
	case StateGameOver, StateWon:
		b.game.Restart()
	}
}

// --- input: keyboard ---

func (b *Board) FocusGained()   {}
func (b *Board) FocusLost()     {}
func (b *Board) TypedRune(rune) {}

func (b *Board) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeySpace:
		b.activate()
	case fyne.KeyP:
		b.game.TogglePause()
	case fyne.KeyR:
		b.game.Restart()
	}
}

func (b *Board) KeyDown(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyLeft, fyne.KeyA:
		b.game.leftHeld = true
	case fyne.KeyRight, fyne.KeyD:
		b.game.rightHeld = true
	}
}

func (b *Board) KeyUp(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyLeft, fyne.KeyA:
		b.game.leftHeld = false
	case fyne.KeyRight, fyne.KeyD:
		b.game.rightHeld = false
	}
}

// boardRenderer maps game state onto canvas objects every frame.
type boardRenderer struct {
	board *Board
	game  *Game

	bg        *canvas.Rectangle
	over      *canvas.Rectangle
	paddle    *canvas.Rectangle
	ball      *canvas.Circle
	bricks    []*canvas.Rectangle
	score     *canvas.Text
	lives     *canvas.Text
	levelText *canvas.Text
	title     *canvas.Text
	subtitle  *canvas.Text
}

func (r *boardRenderer) Layout(size fyne.Size) {
	r.game.Resize(size.Width, size.Height)
	r.bg.Resize(size)
	r.over.Resize(size)
	r.syncBricks()
}

func (r *boardRenderer) MinSize() fyne.Size { return fyne.NewSize(320, 400) }

func (r *boardRenderer) Destroy() {}

// syncBricks makes the rectangle pool match the current brick count. The pool is
// rebuilt only when a new level changes the number of bricks.
func (r *boardRenderer) syncBricks() {
	if len(r.bricks) == len(r.game.bricks) {
		return
	}
	r.bricks = make([]*canvas.Rectangle, len(r.game.bricks))
	for i := range r.bricks {
		rect := canvas.NewRectangle(r.game.bricks[i].col)
		rect.CornerRadius = 3
		r.bricks[i] = rect
	}
}

func (r *boardRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	for _, br := range r.bricks {
		objs = append(objs, br)
	}
	objs = append(objs, r.paddle, r.ball, r.score, r.lives, r.levelText, r.over, r.title, r.subtitle)
	return objs
}

// Refresh repositions and restyles every object from the model.
func (r *boardRenderer) Refresh() {
	g := r.game
	r.syncBricks()

	for i, b := range g.bricks {
		rect := r.bricks[i]
		if !b.alive {
			rect.Hide()
			continue
		}
		rect.Show()
		rect.FillColor = b.col
		rect.Move(fyne.NewPos(b.x, b.y))
		rect.Resize(fyne.NewSize(b.w, b.h))
	}

	r.paddle.Move(fyne.NewPos(g.paddleX, g.paddleY))
	r.paddle.Resize(fyne.NewSize(g.paddleW, g.paddleH))

	d := g.ballR * 2
	r.ball.Move(fyne.NewPos(g.ballX-g.ballR, g.ballY-g.ballR))
	r.ball.Resize(fyne.NewSize(d, d))

	hud := g.h * 0.03
	r.score.TextSize = hud
	r.score.Text = fmt.Sprintf("SCORE  %d", g.score)
	r.score.Move(fyne.NewPos(g.w*0.04, g.h*0.04))

	r.lives.TextSize = hud
	r.lives.Text = fmt.Sprintf("LIVES  %s", lifeIcons(g.lives))
	r.lives.Resize(fyne.NewSize(g.w*0.92, hud*1.4))
	r.lives.Move(fyne.NewPos(0, g.h*0.04))

	r.levelText.TextSize = hud
	r.levelText.Text = fmt.Sprintf("LEVEL %d", g.level)
	r.levelText.Resize(fyne.NewSize(g.w, hud*1.4))
	r.levelText.Move(fyne.NewPos(0, g.h*0.04))

	r.refreshBanner()

	canvas.Refresh(r.bg)
}

// moveDynamic repositions just the ball and paddle and asks the canvas to
// recomposite. Their textures are unchanged (only position moved), so this is
// far cheaper than Refresh, which reformats text and restyles every brick.
func (r *boardRenderer) moveDynamic() {
	g := r.game
	r.paddle.Move(fyne.NewPos(g.paddleX, g.paddleY))
	r.ball.Move(fyne.NewPos(g.ballX-g.ballR, g.ballY-g.ballR))
	canvas.Refresh(r.paddle)
	canvas.Refresh(r.ball)
}

// refreshBanner shows the centre-screen prompt appropriate to the game state.
func (r *boardRenderer) refreshBanner() {
	g := r.game
	var title, sub string
	// Paused / game-over / win screens veil the board so they read as overlays.
	r.over.Hide()
	switch g.state {
	case StateReady:
		if g.score == 0 && g.level == 1 {
			title = "TideBreaker"
		} else {
			title = fmt.Sprintf("Level %d", g.level)
		}
		sub = "Click or press Space to launch"
	case StatePaused:
		r.over.Show()
		title = "Paused"
		sub = "Press Space to resume"
	case StateGameOver:
		r.over.Show()
		title = "Game Over"
		sub = fmt.Sprintf("Score %d  ·  Space or click to play again", g.score)
	case StateWon:
		r.over.Show()
		title = "You Win!"
		sub = fmt.Sprintf("All %d levels cleared  ·  Score %d  ·  Space to play again", maxLevel, g.score)
	default:
		r.title.Text, r.subtitle.Text = "", ""
		r.title.Hide()
		r.subtitle.Hide()
		return
	}

	r.title.Show()
	r.subtitle.Show()
	r.title.Text = title
	r.title.TextSize = g.h * 0.06
	r.title.Resize(fyne.NewSize(g.w, r.title.TextSize*1.4))
	r.title.Move(fyne.NewPos(0, g.h*0.42))

	r.subtitle.Text = sub
	r.subtitle.TextSize = g.h * 0.028
	r.subtitle.Resize(fyne.NewSize(g.w, r.subtitle.TextSize*1.4))
	r.subtitle.Move(fyne.NewPos(0, g.h*0.42+r.title.TextSize*1.5))
}

func lifeIcons(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "● "
	}
	if s == "" {
		s = "—"
	}
	return s
}

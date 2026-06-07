package main

import "testing"

// newTestGame returns a game with a fixed board laid out, ready to play.
func newTestGame() *Game {
	g := NewGame()
	g.Resize(640, 800) // first Resize initialises the level
	return g
}

func TestClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want float32 }{
		{5, 0, 10, 5},
		{-3, 0, 10, 0},
		{99, 0, 10, 10},
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%v,%v,%v)=%v want %v", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

func TestSetupLevel(t *testing.T) {
	g := newTestGame()
	if got := len(g.bricks); got != brickCols*brickRows {
		t.Fatalf("brick count = %d, want %d", got, brickCols*brickRows)
	}
	if g.aliveBricks() != brickCols*brickRows {
		t.Fatalf("all bricks should start alive")
	}
	if g.state != StateReady {
		t.Fatalf("state = %v, want StateReady", g.state)
	}
}

func TestLaunchOnlyFromReady(t *testing.T) {
	g := newTestGame()
	g.Launch()
	if g.state != StatePlaying {
		t.Fatalf("Launch should move Ready -> Playing")
	}
	if g.ballVY >= 0 {
		t.Fatalf("ball should travel upward after launch, vy=%v", g.ballVY)
	}
	// A second launch while Playing must be a no-op.
	vy := g.ballVY
	g.Launch()
	if g.ballVY != vy {
		t.Fatalf("Launch while Playing changed velocity")
	}
}

func TestSideWallBounce(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	g.ballX, g.ballY = 2, 400
	g.ballVX, g.ballVY = -100, 0
	g.moveBall(0.05)
	if g.ballVX <= 0 {
		t.Fatalf("ball should bounce off the left wall, vx=%v", g.ballVX)
	}
	if g.ballX < 0 {
		t.Fatalf("ball escaped the left wall, x=%v", g.ballX)
	}
}

func TestBottomLosesLife(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	g.lives = 2
	g.ballX, g.ballY = 320, g.h+50
	g.ballVX, g.ballVY = 0, 100
	g.moveBall(0.05)
	if g.lives != 1 {
		t.Fatalf("lives = %d, want 1", g.lives)
	}
	if g.state != StateReady {
		t.Fatalf("state = %v, want StateReady after losing a life", g.state)
	}
}

func TestLastLifeEndsGame(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	g.lives = 1
	g.ballY = g.h + 50
	g.ballVY = 100
	g.moveBall(0.05)
	if g.state != StateGameOver {
		t.Fatalf("state = %v, want StateGameOver", g.state)
	}
}

func TestBrickBreakScoresAndBounces(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	b := g.bricks[0]
	b.points = 40
	// Place the ball just below the brick, moving up into it.
	g.ballX = b.x + b.w/2
	g.ballY = b.y + b.h + g.ballR - 1
	g.ballVX, g.ballVY = 0, -300
	g.moveBall(0.0) // no integration; resolve the existing overlap
	if b.alive {
		t.Fatalf("brick should be destroyed")
	}
	if g.score != 40 {
		t.Fatalf("score = %d, want 40", g.score)
	}
	if g.ballVY <= 0 {
		t.Fatalf("ball should bounce downward off the brick, vy=%v", g.ballVY)
	}
}

func TestClearingLevelAdvances(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	for _, b := range g.bricks {
		b.alive = false
	}
	g.bricks[len(g.bricks)-1].alive = true
	last := g.bricks[len(g.bricks)-1]
	g.ballX = last.x + last.w/2
	g.ballY = last.y + last.h + g.ballR - 1
	g.ballVY = -300
	g.moveBall(0.0)
	if g.level != 2 {
		t.Fatalf("level = %d, want 2 after clearing the board", g.level)
	}
	if g.aliveBricks() != brickCols*brickRows {
		t.Fatalf("new level should refill the board")
	}
}

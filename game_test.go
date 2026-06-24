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
	if want := patternBrickCount(levels[1]); g.aliveBricks() != want {
		t.Fatalf("new level should refill the board: got %d bricks, want %d", g.aliveBricks(), want)
	}
}

// patternBrickCount counts the non-empty cells in a level layout.
func patternBrickCount(pattern []string) int {
	n := 0
	for _, line := range pattern {
		for col := 0; col < brickCols && col < len(line); col++ {
			if c := line[col]; c != '.' && c != ' ' {
				n++
			}
		}
	}
	return n
}

// hitBrick drives the ball up into the given brick once and resolves the
// collision in place (no integration).
func hitBrick(g *Game, b *Brick) {
	g.ballX = b.x + b.w/2
	g.ballY = b.y + b.h + g.ballR - 1
	g.ballVX, g.ballVY = 0, -300
	g.moveBall(0.0)
}

func TestMultiHitBrickTakesSeveralBlows(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	b := makeBrick('3', 0) // gold, three hits
	b.x, b.y, b.w, b.h = 300, 200, 40, 20
	g.bricks = []*Brick{b}

	for i := 0; i < 2; i++ {
		hitBrick(g, b)
		if !b.alive {
			t.Fatalf("gold brick should survive blow %d", i+1)
		}
		if g.score != 0 {
			t.Fatalf("no score until a tough brick breaks, got %d", g.score)
		}
	}
	hitBrick(g, b)
	if b.alive {
		t.Fatalf("gold brick should break on the third blow")
	}
	if g.score != goldPoints {
		t.Fatalf("score = %d, want %d", g.score, goldPoints)
	}
}

func TestUnbreakableBrickNeverBreaks(t *testing.T) {
	g := newTestGame()
	g.state = StatePlaying
	wall := makeBrick('#', 0)
	wall.x, wall.y, wall.w, wall.h = 300, 200, 40, 20
	g.bricks = []*Brick{wall}

	// Drive the ball into the wall and resolve the collision directly, so the
	// empty-of-breakables board doesn't auto-advance the level first.
	g.ballX = wall.x + wall.w/2
	g.ballY = wall.y + wall.h + g.ballR - 1
	g.ballVX, g.ballVY = 0, -300
	g.brickCollisions()

	if !wall.alive {
		t.Fatalf("unbreakable wall should never break")
	}
	if g.score != 0 {
		t.Fatalf("unbreakable wall should not score, got %d", g.score)
	}
	if g.ballVY <= 0 {
		t.Fatalf("ball should still bounce off the wall, vy=%v", g.ballVY)
	}
	// Unbreakable bricks never count toward clearing the board.
	if g.breakableAlive() != 0 {
		t.Fatalf("breakableAlive = %d, want 0", g.breakableAlive())
	}
}

func TestClearingFinalLevelWins(t *testing.T) {
	g := newTestGame()
	g.level = maxLevel
	g.setupLevel()
	g.state = StatePlaying
	// Find one breakable brick, clear all the others, then knock it out.
	var last *Brick
	for _, b := range g.bricks {
		if !b.unbreakable {
			last = b
		}
	}
	for _, b := range g.bricks {
		if b != last && !b.unbreakable {
			b.alive = false
		}
	}
	last.hits = 1
	hitBrick(g, last)
	if g.state != StateWon {
		t.Fatalf("state = %v, want StateWon after clearing the last level", g.state)
	}
}

package main

import (
	"errors"
	"runtime"
)

func getFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}

	return frame
}

type stopRope struct {
	ropeHolders  int
	holdChan     chan int
	cutChan      chan interface{}
	releasedChan chan interface{}
	isCut        bool
	isReleased   bool
}

// Simple interface to make it easy to tear down a pool of goroutines.
// The metaphor of a rope is used:
// - goroutines Hold() the rope when they start
// - they will then check the WaitCut() channel to see if the rope has been cut
// - when a stop condition is met and goroutines must stop, the rope can be Cut()
// - goroutines must Release() the rope
// - the "master" will wait for all the goroutines with WaitReleased() for joining them
//
// I don't know if there's a better way to accomplish this, but I like the metaphor. Also I haven't spent too much time
// checking thread-safety, hopefully it is 🤷 lol
type StopRope interface {
	Hold() error
	Release()
	Cut()
	WaitCut() <-chan interface{}
	WaitReleased() <-chan interface{}
	IsCut() bool
	IsReleased() bool
}

func NewRope() StopRope {
	rope := &stopRope{
		ropeHolders:  0,
		holdChan:     make(chan int),
		cutChan:      make(chan interface{}),
		releasedChan: make(chan interface{}),
		isCut:        false,
		isReleased:   false,
	}
	go rope.ropeWatcher()
	return rope
}

func (rope *stopRope) ropeWatcher() {
	for {
		rope.ropeHolders += <-rope.holdChan

		if rope.ropeHolders == 0 && rope.isCut {
			rope.isReleased = true
			close(rope.holdChan)
			close(rope.releasedChan)
			return
		}
	}
}

func (rope *stopRope) Hold() error {
	if !rope.isReleased && !rope.isCut {
		rope.holdChan <- 1
		return nil
	}
	return errors.New("rope is cut")
}

func (rope *stopRope) Release() {
	if !rope.isReleased {
		rope.holdChan <- -1
	}
}

func (rope *stopRope) Cut() {
	if !rope.isCut {
		close(rope.cutChan)
		rope.isCut = true
	}
}

func (rope *stopRope) WaitCut() <-chan interface{} {
	return rope.cutChan
}

func (rope *stopRope) WaitReleased() <-chan interface{} {
	return rope.releasedChan
}

func (rope *stopRope) IsReleased() bool {
	return rope.isReleased
}

func (rope *stopRope) IsCut() bool {
	return rope.isCut
}

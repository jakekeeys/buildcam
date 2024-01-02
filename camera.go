package main

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)

type Camera struct {
	cachedFrame *Frame
	m           sync.RWMutex
}

func NewCamera() *Camera {
	return &Camera{
		cachedFrame: nil,
		m:           sync.RWMutex{},
	}
}

type Frame struct {
	bytes   []byte
	created time.Time
}

func (c *Camera) GetFrame() (*Frame, error) {
	if c.cachedFrame == nil || c.cachedFrame.created.Before(time.Now().Add(time.Minute*-1)) {
		err := c.updateFrame()
		if err != nil {
			return nil, err
		}
	}

	c.m.RLock()
	defer c.m.RUnlock()

	return &Frame{
		bytes:   bytes.Clone(c.cachedFrame.bytes),
		created: c.cachedFrame.created,
	}, nil
}

func (c *Camera) updateFrame() error {
	tdir, err := os.MkdirTemp("", "*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tdir)

	mp4Path := path.Join(tdir, "frame.mp4")
	_, err = exec.Command("wget", "http://frigate:1984/api/frame.mp4?src=house", "-O", mp4Path).CombinedOutput()
	//println(string(output))
	if err != nil {
		return err
	}

	jpgPath := path.Join(tdir, "frame.jpg")
	_, err = exec.Command("ffmpeg", "-ss", "00:00", "-i", mp4Path, "-vframes", "1", "-q:v", "2", "-update", "1", jpgPath).CombinedOutput()
	//println(string(output))
	if err != nil {
		return err
	}

	jpgBytes, err := os.ReadFile(jpgPath)
	if err != nil {
		return err
	}

	c.m.Lock()
	defer c.m.Unlock()

	c.cachedFrame = &Frame{
		bytes:   jpgBytes,
		created: time.Now(),
	}

	return nil
}

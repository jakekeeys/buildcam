package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"time"
)

const (
	framesDir = "/service/data/frames"
	lapseDir  = "/service/data/lapse"
)

type Timelapse struct {
	c *Camera
	m sync.RWMutex
}

func NewTimelapse(c *Camera) (*Timelapse, error) {
	err := os.MkdirAll(lapseDir, os.ModeDevice)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(framesDir, os.ModeDevice)
	if err != nil {
		return nil, err
	}

	return &Timelapse{
		c: c,
		m: sync.RWMutex{},
	}, nil
}

func (t *Timelapse) saveFrame() error {
	if time.Now().Hour() < 7 || time.Now().Hour() > 18 {
		println("skipping saving new frame")
		return nil
	}

	frame, err := t.c.GetFrame()
	if err != nil {
		return err
	}

	t.m.Lock()
	defer t.m.Unlock()
	f, err := os.Create(path.Join(framesDir, fmt.Sprintf("%s.jpg", frame.created.Format(time.RFC3339))))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(frame.bytes)
	if err != nil {
		return err
	}

	return nil
}

func (t *Timelapse) updateCompleteLapse() error {
	glob := fmt.Sprintf("/service/data/frames/*.jpg")
	err := t.updateLapse(glob, "complete.mp4", "10")
	if err != nil {
		return err
	}

	return nil
}

func (t *Timelapse) updateLatestLapse() error {
	if time.Now().Hour() >= 20 {
		println("skipping latest lapse update")
		return nil
	}

	glob := fmt.Sprintf("/service/data/frames/%s*.jpg", time.Now().Format("2006-01-02"))
	matches, err := filepath.Glob(glob)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return errors.New("no frames for date")
	}

	err = t.updateLapse(glob, fmt.Sprintf("%s.mp4", time.Now().Format("2006-01-02")), "2")
	if err != nil {
		return err
	}

	return nil
}

func (t *Timelapse) updateLapse(filter, lapseName, fps string) error {
	tdir, err := os.MkdirTemp("", "*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tdir)
	lapseTmpPath := path.Join(tdir, lapseName)

	_, err = exec.Command("ffmpeg", "-framerate", fps, "-pattern_type", "glob", "-i", filter, "-s:v", "1920x1080", "-c:v", "libx264", "-crf", "17", "-pix_fmt", "yuv420p", lapseTmpPath).CombinedOutput()
	if err != nil {
		return err
	}

	t.m.Lock()
	defer t.m.Unlock()
	lapsePath := path.Join(lapseDir, lapseName)
	os.Remove(lapsePath)

	src, err := os.Open(lapseTmpPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(lapsePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	fi, err := os.Stat(lapseTmpPath)
	if err != nil {
		return err
	}
	err = os.Chmod(lapsePath, fi.Mode())
	if err != nil {
		return err
	}
	os.Remove(lapseTmpPath)

	return nil
}

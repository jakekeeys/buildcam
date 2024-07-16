package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"time"
)

var (
	camera    *Camera
	timelapse *Timelapse
	apiToken  string
)

func main() {
	apiToken = os.Getenv("TOKEN")
	camera = NewCamera()
	tl, err := NewTimelapse(camera)
	if err != nil {
		log.Printf("error init %+v\n", err)

	}
	timelapse = tl

	r := gin.Default()
	r.GET("/", rootHandler)
	r.GET("/snapshot", snapshotHandler)
	r.GET("/timelapse", timelapseHandler)

	r.StaticFS("/stream", http.Dir("./assets/stream"))
	r.GET("/streamws", func(c *gin.Context) {
		gortc, err := url.Parse("http://frigate:1984/api/ws?src=house")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		token := c.Query("token")
		if token != apiToken {
			c.String(http.StatusUnauthorized, "bad token")
			return
		}

		proxy := httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.Out.URL = gortc
			},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	c := cron.New(cron.WithLocation(time.UTC))
	_, err = c.AddFunc("*/15 * * * *", func() {
		println("saving new frame")
		err := timelapse.saveFrame()
		if err != nil {
			log.Printf("error saving frame %+v\n", err)
		}
	})
	if err != nil {
		log.Printf("error init %+v\n", err)
	}
	_, err = c.AddFunc("0 * * * *", func() {
		println("updating latest lapse")
		err := timelapse.updateLatestLapse()
		if err != nil {
			log.Printf("error updating latest lapse %+v\n", err)
		}
	})
	if err != nil {
		log.Printf("error init %+v\n", err)
	}
	_, err = c.AddFunc("0 20 * * *", func() {
		println("updating complete lapse")
		err := timelapse.updateCompleteLapse()
		if err != nil {
			log.Printf("error updating complete lapse %+v\n", err)
		}
	})
	if err != nil {
		log.Printf("error init %+v\n", err)
	}
	c.Start()

	err = r.Run(":8080")
	if err != nil {
		panic(err)
	}
}

func rootHandler(c *gin.Context) {
	c.String(http.StatusOK, `/snapshot - returns a live snapshotHandler that can be refreshed up to every minute
/timelapse - returns a timelapse for the current day which is refreshed every hour
  params
    range
      "" - will return the timelapse for the current day
      "complete" - will return a complete timelapse including all frames
      "2023-05-15" will return the timelapse for the provided date`)
}

func timelapseHandler(c *gin.Context) {
	token := c.Query("token")
	if token != apiToken {
		c.String(http.StatusUnauthorized, "bad token")
		return
	}

	lapsePath := ""
	lapseRange := c.Query("range")
	switch lapseRange {
	case "complete":
		lapsePath = path.Join(lapseDir, "complete.mp4")
	case "":
		lapsePath = path.Join(lapseDir, fmt.Sprintf("%s.mp4", time.Now().Format("2006-01-02")))
	default:
		lapseDate, err := time.Parse("2006-01-02", lapseRange)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		lapsePath = path.Join(lapseDir, fmt.Sprintf("%s.mp4", lapseDate.Format("2006-01-02")))
	}

	_, err := os.Stat(lapsePath)
	if errors.Is(err, os.ErrNotExist) {
		c.String(http.StatusNotFound, "no data for date")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "video/mp4")
	timelapse.m.RLock()
	http.ServeFile(c.Writer, c.Request, lapsePath)
	timelapse.m.RUnlock()
}

func snapshotHandler(c *gin.Context) {
	token := c.Query("token")
	if token != apiToken {
		c.String(http.StatusUnauthorized, "bad token")
		return
	}

	frame, err := camera.GetFrame()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "image/jpeg")
	_, err = c.Writer.Write(frame.bytes)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
}

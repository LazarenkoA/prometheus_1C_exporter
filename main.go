package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"
)

type Iexplorer interface {
	StartExplore()
}

type Metrics struct {
	explorers []Iexplorer
}

func main() {
	var port string
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.Parse()

	siteMux := http.NewServeMux()
	metric := new(Metrics)
	metric.append(new(ExplorerClientLic).Construct(siteMux, time.Second*10))
	for _, ex := range metric.explorers {
		go ex.StartExplore()
	}

	fmt.Println("starting server at :", port)
	http.ListenAndServe(":"+port, siteMux)
}

func (this *Metrics) append(ex Iexplorer) {
	this.explorers = append(this.explorers, ex)
}

// go build -ldflags "-s -w" - билд чутка меньше размером

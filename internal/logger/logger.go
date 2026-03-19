package logger

import (
	"log"
	"os"
)

var (
	Sensor     *log.Logger
	Atuador    *log.Logger
	Integrador *log.Logger
)

func init() {
	Sensor = log.New(os.Stdout, "[SENSOR] ", log.LstdFlags)
	Atuador = log.New(os.Stdout, "[ATUADOR] ", log.LstdFlags)
	Integrador = log.New(os.Stdout, "[INTEGRADOR] ", log.LstdFlags)
}

package models

import "time"

// MensagemSensor representa os dados enviados via UDP (Telemetria)
type MensagemSensor struct {
	NodeID        string    `json:"node_id"`
	SensorID      string    `json:"sensor_id"`
	Tipo          string    `json:"tipo"`
	Valor         float64   `json:"valor"`
	Unidade       string    `json:"unidade"`
	Timestamp     time.Time `json:"timestamp"`
	StatusLeitura string    `json:"status_leitura"`
}

// ComandoAtuador representa as ordens enviadas via TCP (Controle)
type ComandoAtuador struct {
	NodeID            string    `json:"node_id"`
	AtuadorID         string    `json:"atuador_id"`
	Comando           string    `json:"comando"`
	MotivoAcionamento string    `json:"motivo_acionamento"`
	TimestampOrdem    time.Time `json:"timestamp_ordem"`
}

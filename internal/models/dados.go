package models

import "time"

// MensagemSensor representa os dados enviados via UDP
type MensagemSensor struct {
	EstufaID      string    `json:"estufa_id"`
	SensorID      string    `json:"sensor_id"`
	Tipo          string    `json:"tipo"`
	Valor         float64   `json:"valor"`
	Unidade       string    `json:"unidade"`
	Timestamp     time.Time `json:"timestamp"`
	StatusLeitura string    `json:"status_leitura"`
}

// ComandoAtuador representa as ordens enviadas via TCP
type ComandoAtuador struct {
	EstufaID          string    `json:"estufa_id"`
	AtuadorID         string    `json:"atuador_id"`
	Comando           string    `json:"comando"`
	MotivoAcionamento string    `json:"motivo_acionamento"`
	TimestampOrdem    time.Time `json:"timestamp_ordem"`
}

package models

import "time"

// MensagemSensor representa os dados de telemetria enviados via UDP pelos sensores.
type MensagemSensor struct {
	NodeID        string    `json:"node_id"`
	SensorID      string    `json:"sensor_id"`
	Tipo          string    `json:"tipo"`    // umidade | temperatura | luminosidade | amonia | racao | agua
	Valor         float64   `json:"valor"`
	Unidade       string    `json:"unidade"` // % | C | Lux | ppm
	Timestamp     time.Time `json:"timestamp"`
	StatusLeitura string    `json:"status_leitura"` // normal | alerta | erro
}

// ComandoAtuador representa as ordens de controle enviadas via TCP (servidor → atuadores).
type ComandoAtuador struct {
	NodeID            string    `json:"node_id"`
	AtuadorID         string    `json:"atuador_id"`
	Comando           string    `json:"comando"`            // LIGAR | DESLIGAR
	MotivoAcionamento string    `json:"motivo_acionamento"` // umidade_critica | manual_dashboard | etc.
	TimestampOrdem    time.Time `json:"timestamp_ordem"`
}

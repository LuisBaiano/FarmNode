package models

import "time"

// Dados de telemetria enviados pelos sensores via UDP.
// Sensores enviam apenas dados crus.
type MensagemSensor struct {
	NodeID        string    `json:"node_id"`
	SensorID      string    `json:"sensor_id"`
	Tipo          string    `json:"tipo"` // umidade | temperatura | luminosidade | amonia | racao | agua
	Valor         float64   `json:"valor"`
	Unidade       string    `json:"unidade"` // % | C | Lux | ppm
	Timestamp     time.Time `json:"timestamp"`
	StatusLeitura string    `json:"status_leitura"` // normal
}

// Registro inicial do atuador no servidor TCP.

type RegistroAtuador struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
}

// Comando de controle enviado do servidor ao atuador.

type ComandoAtuador struct {
	NodeID            string    `json:"node_id"`
	AtuadorID         string    `json:"atuador_id"`
	Comando           string    `json:"comando"`            // LIGAR | DESLIGAR
	MotivoAcionamento string    `json:"motivo_acionamento"` // umidade_baixa | manual_dashboard | etc.
	TimestampOrdem    time.Time `json:"timestamp_ordem"`
}

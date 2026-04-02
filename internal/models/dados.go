package models

import "time"

// MensagemSensor representa os dados de telemetria enviados via UDP pelos sensores.
// Sensores enviam APENAS dados crus — sem interpretacao, sem logs de eventos.
type MensagemSensor struct {
	NodeID        string    `json:"node_id"`
	SensorID      string    `json:"sensor_id"`
	Tipo          string    `json:"tipo"`     // umidade | temperatura | luminosidade | amonia | racao | agua
	Valor         float64   `json:"valor"`
	Unidade       string    `json:"unidade"`  // % | C | Lux | ppm
	Timestamp     time.Time `json:"timestamp"`
	StatusLeitura string    `json:"status_leitura"` // normal
}

// RegistroAtuador e a primeira mensagem enviada pelo atuador ao conectar
// no servidor via TCP porta 6000. Identifica qual atuador esta se registrando.
type RegistroAtuador struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
}

// ComandoAtuador representa as ordens de controle enviadas via TCP
// do servidor para o atuador (pelo canal persistente registrado em :6000).
type ComandoAtuador struct {
	NodeID            string    `json:"node_id"`
	AtuadorID         string    `json:"atuador_id"`
	Comando           string    `json:"comando"`            // LIGAR | DESLIGAR
	MotivoAcionamento string    `json:"motivo_acionamento"` // umidade_baixa | manual_dashboard | etc.
	TimestampOrdem    time.Time `json:"timestamp_ordem"`
}

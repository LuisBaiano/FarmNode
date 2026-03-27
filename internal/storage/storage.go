package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"FarmNode/internal/models"
)

const (
	LogsDir         = "logs"
	SensorLogsFile  = "logs/sensor_logs.json"
	AtuadorLogsFile = "logs/atuador_logs.json"
	AlertasFile     = "logs/alertas.json"
	MaxFileSize     = 5 * 1024 * 1024 // 5MB por arquivo
	BufferSize      = 10
)

var (
	mu            sync.Mutex
	sensorBuffer  []SensorLog
	atuadorBuffer []AtuadorLog
)

// ── Structs ───────────────────────────────────────────────────────────────────

type SensorLog struct {
	NodeID        string  `json:"node_id"`
	SensorID      string  `json:"sensor_id"`
	Tipo          string  `json:"tipo"`
	Valor         float64 `json:"valor"`
	Unidade       string  `json:"unidade"`
	Timestamp     string  `json:"timestamp"`
	StatusLeitura string  `json:"status_leitura"`
}

type AtuadorLog struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
	Comando   string `json:"comando"`
	Motivo    string `json:"motivo"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// AlertaLog representa um evento de alerta gerado pelo sistema
type AlertaLog struct {
	ID        string  `json:"id"`
	NodeID    string  `json:"node_id"`
	Tipo      string  `json:"tipo"`     // tipo do sensor que gerou o alerta
	Valor     float64 `json:"valor"`    // valor no momento do alerta
	Mensagem  string  `json:"mensagem"` // descrição legível
	Timestamp string  `json:"timestamp"`
	Nivel     string  `json:"nivel"` // "aviso" | "critico"
	Ack       bool    `json:"ack"`   // true = operador reconheceu o alerta
}

type SensorLogsData struct {
	Logs []SensorLog `json:"logs"`
}

type AtuadorLogsData struct {
	Logs []AtuadorLog `json:"logs"`
}

type AlertasData struct {
	Alertas []AlertaLog `json:"alertas"`
}

// ── Inicialização ─────────────────────────────────────────────────────────────

func InitDB(_ string) error {
	if err := os.MkdirAll(LogsDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de logs: %v", err)
	}

	sensorBuffer = make([]SensorLog, 0, BufferSize)
	atuadorBuffer = make([]AtuadorLog, 0, BufferSize)

	defaults := []struct {
		file string
		data interface{}
	}{
		{SensorLogsFile, SensorLogsData{Logs: []SensorLog{}}},
		{AtuadorLogsFile, AtuadorLogsData{Logs: []AtuadorLog{}}},
		{AlertasFile, AlertasData{Alertas: []AlertaLog{}}},
	}

	for _, d := range defaults {
		if !fileExists(d.file) {
			if err := saveJSON(d.file, d.data); err != nil {
				return fmt.Errorf("erro ao criar %s: %v", d.file, err)
			}
		}
	}

	log.Printf("✓ Storage inicializado (buffer=%d, max=%dMB)", BufferSize, MaxFileSize/1024/1024)
	return nil
}

// ── Escrita ───────────────────────────────────────────────────────────────────

func LogSensor(sensor models.MensagemSensor) error {
	mu.Lock()
	defer mu.Unlock()

	sensorBuffer = append(sensorBuffer, SensorLog{
		NodeID:        sensor.NodeID,
		SensorID:      sensor.SensorID,
		Tipo:          sensor.Tipo,
		Valor:         sensor.Valor,
		Unidade:       sensor.Unidade,
		Timestamp:     sensor.Timestamp.Format(time.RFC3339),
		StatusLeitura: sensor.StatusLeitura,
	})

	if len(sensorBuffer) >= BufferSize {
		return flushSensorBuffer()
	}
	return nil
}

// LogAtuador faz flush imediato (eventos críticos e pouco frequentes)
func LogAtuador(nodeID, atuadorID, comando, motivo string) error {
	mu.Lock()
	defer mu.Unlock()

	atuadorBuffer = append(atuadorBuffer, AtuadorLog{
		NodeID:    nodeID,
		AtuadorID: atuadorID,
		Comando:   comando,
		Motivo:    motivo,
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "executado",
	})
	return flushAtuadorBuffer()
}

// LogAlerta registra um evento de alerta no arquivo alertas.json
func LogAlerta(nodeID, tipo string, valor float64, mensagem, nivel string) error {
	mu.Lock()
	defer mu.Unlock()

	var data AlertasData
	if err := loadJSON(AlertasFile, &data); err != nil {
		data = AlertasData{Alertas: []AlertaLog{}}
	}

	id := fmt.Sprintf("%s_%s_%d", nodeID, tipo, time.Now().UnixNano())
	data.Alertas = append(data.Alertas, AlertaLog{
		ID:        id,
		NodeID:    nodeID,
		Tipo:      tipo,
		Valor:     valor,
		Mensagem:  mensagem,
		Timestamp: time.Now().Format(time.RFC3339),
		Nivel:     nivel,
		Ack:       false,
	})

	if len(data.Alertas) > 1000 {
		data.Alertas = data.Alertas[len(data.Alertas)-1000:]
	}

	return saveJSON(AlertasFile, data)
}

// AckAlerta marca um alerta como reconhecido pelo operador
func AckAlerta(id string) error {
	mu.Lock()
	defer mu.Unlock()

	var data AlertasData
	if err := loadJSON(AlertasFile, &data); err != nil {
		return err
	}

	for i := range data.Alertas {
		if data.Alertas[i].ID == id {
			data.Alertas[i].Ack = true
			break
		}
	}

	return saveJSON(AlertasFile, data)
}

// ── Queries ───────────────────────────────────────────────────────────────────

func GetSensorDataByType(tipoSensor string, horas int) ([]map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)
	var results []map[string]interface{}

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 5000; i-- {
		l := data.Logs[i]
		if l.Tipo != tipoSensor {
			continue
		}
		t, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil || !t.After(cutoff) {
			continue
		}
		results = append(results, map[string]interface{}{
			"node_id":        l.NodeID,
			"sensor_id":      l.SensorID,
			"tipo":           l.Tipo,
			"valor":          l.Valor,
			"unidade":        l.Unidade,
			"timestamp":      l.Timestamp,
			"status_leitura": l.StatusLeitura,
		})
	}
	return results, nil
}

func GetLatestSensorValue(tipoSensor string) (map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	for i := len(data.Logs) - 1; i >= 0; i-- {
		l := data.Logs[i]
		if l.Tipo == tipoSensor {
			return map[string]interface{}{
				"tipo": l.Tipo, "valor": l.Valor, "unidade": l.Unidade,
				"timestamp": l.Timestamp, "node_id": l.NodeID, "sensor_id": l.SensorID,
			}, nil
		}
	}
	return map[string]interface{}{"tipo": tipoSensor, "valor": 0.0, "unidade": ""}, nil
}

func GetAtuadorHistory(atuadorID string, horas int) ([]map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)
	var results []map[string]interface{}

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 500; i-- {
		l := data.Logs[i]
		if l.AtuadorID != atuadorID {
			continue
		}
		t, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil || !t.After(cutoff) {
			continue
		}
		results = append(results, map[string]interface{}{
			"node_id": l.NodeID, "atuador_id": l.AtuadorID,
			"comando": l.Comando, "motivo": l.Motivo,
			"timestamp": l.Timestamp, "status": l.Status,
		})
	}
	return results, nil
}

func GetAllAtuadorHistory(horas int) ([]map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)
	var results []map[string]interface{}

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 1000; i-- {
		l := data.Logs[i]
		t, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil || !t.After(cutoff) {
			continue
		}
		results = append(results, map[string]interface{}{
			"node_id": l.NodeID, "atuador": l.AtuadorID,
			"acao": l.Comando, "motivo": l.Motivo, "timestamp": l.Timestamp,
		})
	}
	return results, nil
}

// GetAlertas retorna alertas. Se apenasAtivos=true, retorna somente os não reconhecidos.
func GetAlertas(apenasAtivos bool) ([]AlertaLog, error) {
	mu.Lock()
	defer mu.Unlock()

	var data AlertasData
	if err := loadJSON(AlertasFile, &data); err != nil {
		return []AlertaLog{}, nil
	}

	if !apenasAtivos {
		// Retorna os 100 mais recentes
		if len(data.Alertas) > 100 {
			return data.Alertas[len(data.Alertas)-100:], nil
		}
		return data.Alertas, nil
	}

	var ativos []AlertaLog
	for _, a := range data.Alertas {
		if !a.Ack {
			ativos = append(ativos, a)
		}
	}
	return ativos, nil
}

func GetSensorStats(sensorID string, horas int) (map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)
	var valores []float64

	for _, l := range data.Logs {
		if l.SensorID != sensorID {
			continue
		}
		t, err := time.Parse(time.RFC3339, l.Timestamp)
		if err == nil && t.After(cutoff) {
			valores = append(valores, l.Valor)
		}
	}

	if len(valores) == 0 {
		return map[string]interface{}{
			"sensor_id": sensorID, "total_leituras": 0,
			"valor_medio": 0.0, "valor_minimo": 0.0,
			"valor_maximo": 0.0, "desvio_padrao": 0.0,
		}, nil
	}

	sum, min, max := 0.0, valores[0], valores[0]
	for _, v := range valores {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	mean := sum / float64(len(valores))
	variance := 0.0
	for _, v := range valores {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(valores))

	return map[string]interface{}{
		"sensor_id": sensorID, "total_leituras": len(valores),
		"valor_medio": mean, "valor_minimo": min,
		"valor_maximo": max, "desvio_padrao": math.Sqrt(variance),
	}, nil
}

func GetAtuadorStats(atuadorID string, horas int) (map[string]interface{}, error) {
	mu.Lock()
	defer mu.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)
	total, ligado, desligado := 0, 0, 0

	for _, l := range data.Logs {
		if l.AtuadorID != atuadorID {
			continue
		}
		t, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil || !t.After(cutoff) {
			continue
		}
		total++
		if l.Comando == "LIGAR" {
			ligado++
		} else if l.Comando == "DESLIGAR" {
			desligado++
		}
	}

	return map[string]interface{}{
		"atuador_id": atuadorID, "total_acionamentos": total,
		"vezes_ligado": ligado, "vezes_desligado": desligado,
	}, nil
}

// CloseDB grava buffers pendentes
func CloseDB() error {
	mu.Lock()
	defer mu.Unlock()

	if err := flushSensorBuffer(); err != nil {
		log.Printf("[STORAGE] Erro ao gravar sensores: %v", err)
	}
	if err := flushAtuadorBuffer(); err != nil {
		log.Printf("[STORAGE] Erro ao gravar atuadores: %v", err)
	}
	log.Println("✓ Storage finalizado")
	return nil
}

// ── Buffers internos ──────────────────────────────────────────────────────────

func flushSensorBuffer() error {
	if len(sensorBuffer) == 0 {
		return nil
	}
	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		data = SensorLogsData{Logs: []SensorLog{}}
	}
	data.Logs = append(data.Logs, sensorBuffer...)
	if fileSize(SensorLogsFile) > MaxFileSize {
		if err := rotateFile(SensorLogsFile); err != nil {
			log.Printf("[STORAGE] Erro ao rotacionar sensor_logs: %v", err)
		}
		data = SensorLogsData{Logs: sensorBuffer}
	}
	if len(data.Logs) > 10000 {
		data.Logs = data.Logs[len(data.Logs)-10000:]
	}
	err := saveJSON(SensorLogsFile, data)
	sensorBuffer = sensorBuffer[:0]
	return err
}

func flushAtuadorBuffer() error {
	if len(atuadorBuffer) == 0 {
		return nil
	}
	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		data = AtuadorLogsData{Logs: []AtuadorLog{}}
	}
	data.Logs = append(data.Logs, atuadorBuffer...)
	if fileSize(AtuadorLogsFile) > MaxFileSize {
		if err := rotateFile(AtuadorLogsFile); err != nil {
			log.Printf("[STORAGE] Erro ao rotacionar atuador_logs: %v", err)
		}
		data = AtuadorLogsData{Logs: atuadorBuffer}
	}
	if len(data.Logs) > 5000 {
		data.Logs = data.Logs[len(data.Logs)-5000:]
	}
	err := saveJSON(AtuadorLogsFile, data)
	atuadorBuffer = atuadorBuffer[:0]
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func fileSize(fp string) int64 {
	info, err := os.Stat(fp)
	if err != nil {
		return 0
	}
	return info.Size()
}

func rotateFile(fp string) error {
	if !fileExists(fp) {
		return nil
	}
	ext := fp[strings.LastIndex(fp, "."):]
	name := fp[:len(fp)-len(ext)]
	return os.Rename(fp, fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102_150405"), ext))
}

func loadJSON(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func saveJSON(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

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
	MaxFileSize     = 5 * 1024 * 1024 // 5MB por arquivo
	BufferSize      = 10              // Acumula N logs antes de escrever
)

var (
	mu            sync.Mutex
	sensorBuffer  []SensorLog
	atuadorBuffer []AtuadorLog
)

// SensorLog representa um log de sensor
type SensorLog struct {
	NodeID        string  `json:"node_id"`
	SensorID      string  `json:"sensor_id"`
	Tipo          string  `json:"tipo"`
	Valor         float64 `json:"valor"`
	Unidade       string  `json:"unidade"`
	Timestamp     string  `json:"timestamp"`
	StatusLeitura string  `json:"status_leitura"`
}

// AtuadorLog representa um log de atuador
type AtuadorLog struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
	Comando   string `json:"comando"`
	Motivo    string `json:"motivo"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// SensorLogsData contém lista de logs de sensores
type SensorLogsData struct {
	Logs []SensorLog `json:"logs"`
}

// AtuadorLogsData contém lista de logs de atuadores
type AtuadorLogsData struct {
	Logs []AtuadorLog `json:"logs"`
}

// InitDB inicializa o sistema de logs
func InitDB(_ string) error {
	if err := os.MkdirAll(LogsDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de logs: %v", err)
	}

	sensorBuffer = make([]SensorLog, 0, BufferSize)
	atuadorBuffer = make([]AtuadorLog, 0, BufferSize)

	if !fileExists(SensorLogsFile) {
		if err := saveJSON(SensorLogsFile, SensorLogsData{Logs: []SensorLog{}}); err != nil {
			return fmt.Errorf("erro ao criar sensor_logs.json: %v", err)
		}
	}
	if !fileExists(AtuadorLogsFile) {
		if err := saveJSON(AtuadorLogsFile, AtuadorLogsData{Logs: []AtuadorLog{}}); err != nil {
			return fmt.Errorf("erro ao criar atuador_logs.json: %v", err)
		}
	}

	log.Printf("✓ Storage inicializado (buffer=%d, max=%dMB)", BufferSize, MaxFileSize/1024/1024)
	return nil
}

// LogSensor registra evento de sensor com buffer
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

// LogAtuador registra evento de atuador (flush imediato — eventos raros e críticos)
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

	// Atuadores: flush imediato (são eventos críticos e pouco frequentes)
	return flushAtuadorBuffer()
}

// flushSensorBuffer escreve buffer de sensores no arquivo (deve ser chamado com mu travado)
func flushSensorBuffer() error {
	if len(sensorBuffer) == 0 {
		return nil
	}

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		data = SensorLogsData{Logs: []SensorLog{}}
	}

	data.Logs = append(data.Logs, sensorBuffer...)

	// Rotacionar se arquivo ficar grande
	if fileSize(SensorLogsFile) > MaxFileSize {
		if err := rotateFile(SensorLogsFile); err != nil {
			log.Printf("[STORAGE] Erro ao rotacionar sensor_logs: %v", err)
		}
		data = SensorLogsData{Logs: sensorBuffer} // começa limpo após rotate
	}

	// Limitar a 10000 registros em memória
	if len(data.Logs) > 10000 {
		data.Logs = data.Logs[len(data.Logs)-10000:]
	}

	err := saveJSON(SensorLogsFile, data)
	sensorBuffer = sensorBuffer[:0]
	return err
}

// flushAtuadorBuffer escreve buffer de atuadores no arquivo (deve ser chamado com mu travado)
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

// CloseDB grava buffers pendentes antes de encerrar
func CloseDB() error {
	mu.Lock()
	defer mu.Unlock()

	if err := flushSensorBuffer(); err != nil {
		log.Printf("[STORAGE] Erro ao gravar buffer de sensores: %v", err)
	}
	if err := flushAtuadorBuffer(); err != nil {
		log.Printf("[STORAGE] Erro ao gravar buffer de atuadores: %v", err)
	}

	log.Println("✓ Storage finalizado")
	return nil
}

// ─── Queries ──────────────────────────────────────────────────────────────────

// GetSensorDataByType retorna dados históricos de sensores por tipo
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

// GetLatestSensorValue retorna o valor mais recente de um tipo de sensor
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
				"tipo":      l.Tipo,
				"valor":     l.Valor,
				"unidade":   l.Unidade,
				"timestamp": l.Timestamp,
				"node_id":   l.NodeID,
				"sensor_id": l.SensorID,
			}, nil
		}
	}
	return map[string]interface{}{"tipo": tipoSensor, "valor": 0.0, "unidade": ""}, nil
}

// GetAtuadorHistory retorna histórico de um atuador específico
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
			"node_id":    l.NodeID,
			"atuador_id": l.AtuadorID,
			"comando":    l.Comando,
			"motivo":     l.Motivo,
			"timestamp":  l.Timestamp,
			"status":     l.Status,
		})
	}
	return results, nil
}

// GetAllAtuadorHistory retorna histórico de todos os atuadores
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
			"node_id":   l.NodeID,
			"atuador":   l.AtuadorID,
			"acao":      l.Comando,
			"timestamp": l.Timestamp,
		})
	}
	return results, nil
}

// GetSensorStats retorna estatísticas de um sensor
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
		"sensor_id":      sensorID,
		"total_leituras": len(valores),
		"valor_medio":    mean,
		"valor_minimo":   min,
		"valor_maximo":   max,
		"desvio_padrao":  math.Sqrt(variance),
	}, nil
}

// GetAtuadorStats retorna estatísticas de acionamento de um atuador
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
		"atuador_id":         atuadorID,
		"total_acionamentos": total,
		"vezes_ligado":       ligado,
		"vezes_desligado":    desligado,
	}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func fileSize(filepath string) int64 {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0
	}
	return info.Size()
}

func rotateFile(filepath string) error {
	if !fileExists(filepath) {
		return nil
	}
	ext := filepath[strings.LastIndex(filepath, "."):]
	name := filepath[:len(filepath)-len(ext)]
	return os.Rename(filepath, fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102_150405"), ext))
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

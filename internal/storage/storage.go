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
	BufferSize      = 5               // Acumula 5 logs antes de escrever
)

var (
	mutex         sync.Mutex
	sensorBuffer  []SensorLog
	atuadorBuffer []AtuadorLog
	sensorCount   int
	atuadorCount  int
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
func InitDB(dbPath string) error {
	err := os.MkdirAll(LogsDir, 0755)
	if err != nil {
		return fmt.Errorf("erro ao criar diretório de logs: %v", err)
	}

	// Inicializar buffers vazios
	sensorBuffer = make([]SensorLog, 0, BufferSize)
	atuadorBuffer = make([]AtuadorLog, 0, BufferSize)
	sensorCount = 0
	atuadorCount = 0

	// Criar arquivos vazios se não existirem
	if !fileExists(SensorLogsFile) {
		data := SensorLogsData{Logs: []SensorLog{}}
		if err := saveJSON(SensorLogsFile, data); err != nil {
			return err
		}
	}

	if !fileExists(AtuadorLogsFile) {
		data := AtuadorLogsData{Logs: []AtuadorLog{}}
		if err := saveJSON(AtuadorLogsFile, data); err != nil {
			return err
		}
	}

	log.Println("✓ Sistema de logs com buffer inicializado (5 valores/gravação)")
	return nil
}

// LogSensor registra evento de sensor com buffer
func LogSensor(sensor models.MensagemSensor) error {
	mutex.Lock()
	defer mutex.Unlock()

	log := SensorLog{
		NodeID:        sensor.NodeID,
		SensorID:      sensor.SensorID,
		Tipo:          sensor.Tipo,
		Valor:         sensor.Valor,
		Unidade:       sensor.Unidade,
		Timestamp:     sensor.Timestamp.Format(time.RFC3339),
		StatusLeitura: sensor.StatusLeitura,
	}

	sensorBuffer = append(sensorBuffer, log)
	sensorCount++

	// Escrever quando buffer atingir o tamanho ou a cada 30 segundos
	if len(sensorBuffer) >= BufferSize {
		return flushSensorBuffer()
	}

	return nil
}

// LogAtuador registra evento de atuador com buffer
func LogAtuador(nodeID, atuadorID, comando, motivo string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log := AtuadorLog{
		NodeID:    nodeID,
		AtuadorID: atuadorID,
		Comando:   comando,
		Motivo:    motivo,
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "executado",
	}

	atuadorBuffer = append(atuadorBuffer, log)
	atuadorCount++

	// Escrever quando buffer atingir o tamanho
	if len(atuadorBuffer) >= BufferSize {
		return flushAtuadorBuffer()
	}

	return nil
}

// flushSensorBuffer escreve buffer de sensores no arquivo
func flushSensorBuffer() error {
	if len(sensorBuffer) == 0 {
		return nil
	}

	// Ler logs existentes
	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		// Se arquivo não existe, criar novo
		data = SensorLogsData{Logs: []SensorLog{}}
	}

	// Adicionar buffer aos logs
	data.Logs = append(data.Logs, sensorBuffer...)

	// Verificar tamanho do arquivo
	if fileSize(SensorLogsFile) > MaxFileSize {
		// Rotacionar arquivo
		if err := rotateFile(SensorLogsFile); err != nil {
			log.Printf("Erro ao rotacionar arquivo de sensores: %v", err)
		}
	}

	// Manter apenas últimos 10000 registros em memória
	if len(data.Logs) > 10000 {
		data.Logs = data.Logs[len(data.Logs)-10000:]
	}

	// Salvar
	err := saveJSON(SensorLogsFile, data)

	// Limpar buffer
	sensorBuffer = sensorBuffer[:0]

	return err
}

// flushAtuadorBuffer escreve buffer de atuadores no arquivo
func flushAtuadorBuffer() error {
	if len(atuadorBuffer) == 0 {
		return nil
	}

	// Ler logs existentes
	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		data = AtuadorLogsData{Logs: []AtuadorLog{}}
	}

	// Adicionar buffer aos logs
	data.Logs = append(data.Logs, atuadorBuffer...)

	// Verificar tamanho do arquivo
	if fileSize(AtuadorLogsFile) > MaxFileSize {
		if err := rotateFile(AtuadorLogsFile); err != nil {
			log.Printf("Erro ao rotacionar arquivo de atuadores: %v", err)
		}
	}

	// Manter apenas últimos 5000 registros
	if len(data.Logs) > 5000 {
		data.Logs = data.Logs[len(data.Logs)-5000:]
	}

	// Salvar
	err := saveJSON(AtuadorLogsFile, data)

	// Limpar buffer
	atuadorBuffer = atuadorBuffer[:0]

	return err
}

// rotateFile cria backup do arquivo quando fica muito grande
func rotateFile(filepath string) error {
	if !fileExists(filepath) {
		return nil
	}

	// Gerar nome com timestamp
	timestamp := time.Now().Format("20060102_150405")
	ext := filepath[strings.LastIndex(filepath, "."):]
	name := filepath[:len(filepath)-len(ext)]
	newPath := fmt.Sprintf("%s_%s%s", name, timestamp, ext)

	return os.Rename(filepath, newPath)
}

// fileSize retorna tamanho do arquivo em bytes
func fileSize(filepath string) int64 {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// fileExists verifica se arquivo existe
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// loadJSON carrega dados de arquivo JSON
func loadJSON(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// saveJSON salva dados em arquivo JSON
func saveJSON(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// GetSensorDataByType retorna dados de sensores por tipo
func GetSensorDataByType(tipoSensor string, horas int) ([]map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

	// Iterear em reverso para pegar os mais recentes
	for i := len(data.Logs) - 1; i >= 0 && len(results) < 5000; i-- {
		log := data.Logs[i]
		if log.Tipo == tipoSensor {
			t, err := time.Parse(time.RFC3339, log.Timestamp)
			if err == nil && t.After(cutoff) {
				results = append(results, map[string]interface{}{
					"node_id":        log.NodeID,
					"sensor_id":      log.SensorID,
					"tipo":           log.Tipo,
					"valor":          log.Valor,
					"unidade":        log.Unidade,
					"timestamp":      log.Timestamp,
					"status_leitura": log.StatusLeitura,
				})
			}
		}
	}

	return results, nil
}

// GetLatestSensorValue retorna valor mais recente de sensor por tipo
func GetLatestSensorValue(tipoSensor string) (map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	// Procurar do final para o início
	for i := len(data.Logs) - 1; i >= 0; i-- {
		log := data.Logs[i]
		if log.Tipo == tipoSensor {
			return map[string]interface{}{
				"tipo":      log.Tipo,
				"valor":     log.Valor,
				"unidade":   log.Unidade,
				"timestamp": log.Timestamp,
				"node_id":   log.NodeID,
				"sensor_id": log.SensorID,
			}, nil
		}
	}

	return map[string]interface{}{
		"tipo":    tipoSensor,
		"valor":   0.0,
		"unidade": "",
	}, nil
}

// GetAtuadorHistory retorna histórico de atuadores
func GetAtuadorHistory(atuadorID string, horas int) ([]map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 500; i-- {
		log := data.Logs[i]
		if log.AtuadorID == atuadorID {
			t, err := time.Parse(time.RFC3339, log.Timestamp)
			if err == nil && t.After(cutoff) {
				results = append(results, map[string]interface{}{
					"node_id":    log.NodeID,
					"atuador_id": log.AtuadorID,
					"comando":    log.Comando,
					"motivo":     log.Motivo,
					"timestamp":  log.Timestamp,
					"status":     log.Status,
				})
			}
		}
	}

	return results, nil
}

// GetAllAtuadorHistory retorna histórico de todos atuadores
func GetAllAtuadorHistory(horas int) ([]map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 1000; i-- {
		log := data.Logs[i]
		t, err := time.Parse(time.RFC3339, log.Timestamp)
		if err == nil && t.After(cutoff) {
			results = append(results, map[string]interface{}{
				"node_id":   log.NodeID,
				"atuador":   log.AtuadorID,
				"acao":      log.Comando,
				"timestamp": log.Timestamp,
			})
		}
	}

	return results, nil
}

// GetSensorStats retorna estatísticas de sensor
func GetSensorStats(sensorID string, horas int) (map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	var valores []float64
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

	for _, log := range data.Logs {
		if log.SensorID == sensorID {
			t, err := time.Parse(time.RFC3339, log.Timestamp)
			if err == nil && t.After(cutoff) {
				valores = append(valores, log.Valor)
			}
		}
	}

	if len(valores) == 0 {
		return map[string]interface{}{
			"sensor_id":      sensorID,
			"total_leituras": 0,
			"valor_medio":    0.0,
			"valor_minimo":   0.0,
			"valor_maximo":   0.0,
			"desvio_padrao":  0.0,
		}, nil
	}

	sum := 0.0
	min := valores[0]
	max := valores[0]

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
	stddev := math.Sqrt(variance)

	return map[string]interface{}{
		"sensor_id":      sensorID,
		"total_leituras": len(valores),
		"valor_medio":    mean,
		"valor_minimo":   min,
		"valor_maximo":   max,
		"desvio_padrao":  stddev,
	}, nil
}

// GetAtuadorStats retorna estatísticas de atuador
func GetAtuadorStats(atuadorID string, horas int) (map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return nil, err
	}

	var totalAcionamentos int
	var vezesLigado int
	var vezesDesligado int
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

	for _, log := range data.Logs {
		if log.AtuadorID == atuadorID {
			t, err := time.Parse(time.RFC3339, log.Timestamp)
			if err == nil && t.After(cutoff) {
				totalAcionamentos++
				if log.Comando == "LIGAR" {
					vezesLigado++
				} else if log.Comando == "DESLIGAR" {
					vezesDesligado++
				}
			}
		}
	}

	return map[string]interface{}{
		"atuador_id":         atuadorID,
		"total_acionamentos": totalAcionamentos,
		"vezes_ligado":       vezesLigado,
		"vezes_desligado":    vezesDesligado,
	}, nil
}

// CloseDB finaliza e grava buffers pendentes
func CloseDB() error {
	mutex.Lock()
	defer mutex.Unlock()

	// Gravar buffers pendentes
	if err := flushSensorBuffer(); err != nil {
		log.Printf("Erro ao gravar buffer de sensores: %v", err)
	}

	if err := flushAtuadorBuffer(); err != nil {
		log.Printf("Erro ao gravar buffer de atuadores: %v", err)
	}

	log.Println("✓ Logs finalizados e gravados com sucesso")
	return nil
}

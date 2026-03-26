package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"FarmNode/internal/models"
)

const (
	SensorLogsFile  = "logs/sensor_logs.json"
	AtuadorLogsFile = "logs/atuador_logs.json"
)

var mutex sync.Mutex

// SensorLog representa um log de sensor salvo
type SensorLog struct {
	NodeID        string  `json:"node_id"`
	SensorID      string  `json:"sensor_id"`
	Tipo          string  `json:"tipo"`
	Valor         float64 `json:"valor"`
	Unidade       string  `json:"unidade"`
	Timestamp     string  `json:"timestamp"`
	StatusLeitura string  `json:"status_leitura"`
}

// AtuadorLog representa um log de atuador salvo
type AtuadorLog struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
	Comando   string `json:"comando"`
	Motivo    string `json:"motivo"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// SensorLogsData contém a lista de logs de sensores
type SensorLogsData struct {
	Logs []SensorLog `json:"logs"`
}

// AtuadorLogsData contém a lista de logs de atuadores
type AtuadorLogsData struct {
	Logs []AtuadorLog `json:"logs"`
}

// fileExists verifica se um arquivo existe
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// loadJSON carrega dados de um arquivo JSON
func loadJSON(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// saveJSON salva dados em um arquivo JSON
func saveJSON(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// InitDB inicializa o diretório de logs JSON
func InitDB(dbPath string) error {
	// Criar diretório logs se não existir
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		return fmt.Errorf("erro ao criar diretório de logs: %v", err)
	}

	// Criar arquivos JSON vazios se não existirem
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

	log.Println("✓ Sistema de logs JSON inicializado com sucesso")
	return nil
}

// LogSensor registra um evento de sensor em JSON
func LogSensor(sensor models.MensagemSensor) error {
	mutex.Lock()
	defer mutex.Unlock()

	// Ler logs existentes
	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return err
	}

	// Adicionar novo log
	log := SensorLog{
		NodeID:        sensor.NodeID,
		SensorID:      sensor.SensorID,
		Tipo:          sensor.Tipo,
		Valor:         sensor.Valor,
		Unidade:       sensor.Unidade,
		Timestamp:     sensor.Timestamp.Format(time.RFC3339),
		StatusLeitura: sensor.StatusLeitura,
	}

	data.Logs = append(data.Logs, log)

	// Manter apenas últimos 10000 registros (para evitar arquivo muito grande)
	if len(data.Logs) > 10000 {
		data.Logs = data.Logs[len(data.Logs)-10000:]
	}

	// Salvar
	return saveJSON(SensorLogsFile, data)
}

// LogAtuador registra um evento de atuador em JSON
func LogAtuador(nodeID, atuadorID, comando, motivo string) error {
	mutex.Lock()
	defer mutex.Unlock()

	// Ler logs existentes
	var data AtuadorLogsData
	if err := loadJSON(AtuadorLogsFile, &data); err != nil {
		return err
	}

	// Adicionar novo log
	log := AtuadorLog{
		NodeID:    nodeID,
		AtuadorID: atuadorID,
		Comando:   comando,
		Motivo:    motivo,
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "executado",
	}

	data.Logs = append(data.Logs, log)

	// Manter apenas últimos 5000 registros
	if len(data.Logs) > 5000 {
		data.Logs = data.Logs[len(data.Logs)-5000:]
	}

	// Salvar
	return saveJSON(AtuadorLogsFile, data)
}

// GetSensorData retorna dados de um sensor em período específico
func GetSensorData(sensorID string, timerango int) ([]map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	cutoff := time.Now().Add(-time.Duration(timerango) * time.Hour)

	for i := len(data.Logs) - 1; i >= 0 && len(results) < 1000; i-- {
		log := data.Logs[i]
		if log.SensorID == sensorID {
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

// GetSensorDataByType retorna dados de sensores do mesmo tipo
func GetSensorDataByType(tipoSensor string, horas int) ([]map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	cutoff := time.Now().Add(-time.Duration(horas) * time.Hour)

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

// GetAtuadorHistory retorna histórico de um atuador
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

// GetSensorStats retorna estatísticas de um sensor
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

	// Calcular estatísticas
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

	// Desvio padrão
	variance := 0.0
	for _, v := range valores {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(valores))
	stddev := math.Sqrt(variance)

	stats := map[string]interface{}{
		"sensor_id":      sensorID,
		"total_leituras": len(valores),
		"valor_medio":    mean,
		"valor_minimo":   min,
		"valor_maximo":   max,
		"desvio_padrao":  stddev,
	}

	return stats, nil
}

// GetAtuadorStats retorna estatísticas de um atuador
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

	stats := map[string]interface{}{
		"atuador_id":         atuadorID,
		"total_acionamentos": totalAcionamentos,
		"vezes_ligado":       vezesLigado,
		"vezes_desligado":    vezesDesligado,
	}

	return stats, nil
}

// GetLatestSensorValue retorna o valor mais recente de um tipo de sensor
func GetLatestSensorValue(tipoSensor string) (map[string]interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var data SensorLogsData
	if err := loadJSON(SensorLogsFile, &data); err != nil {
		return nil, err
	}

	// Encontrar o log mais recente do tipo
	var latest *SensorLog
	var latestTime time.Time

	for i := len(data.Logs) - 1; i >= 0; i-- {
		log := data.Logs[i]
		if log.Tipo == tipoSensor {
			t, err := time.Parse(time.RFC3339, log.Timestamp)
			if err == nil {
				if latest == nil || t.After(latestTime) {
					latest = &data.Logs[i]
					latestTime = t
				}
			}
		}
	}

	if latest == nil {
		return map[string]interface{}{
			"tipo":    tipoSensor,
			"valor":   0.0,
			"unidade": "",
		}, nil
	}

	return map[string]interface{}{
		"tipo":      latest.Tipo,
		"valor":     latest.Valor,
		"unidade":   latest.Unidade,
		"timestamp": latest.Timestamp,
		"node_id":   latest.NodeID,
		"sensor_id": latest.SensorID,
	}, nil
}

// GetAllAtuadorHistory retorna histórico de todos os atuadores
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

// CloseDB fecha o sistema de logs (no-op para JSON)
func CloseDB() error {
	return nil
}

package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"cyrbaby/pkg/protocol"
)

func handleFileGet(req protocol.FileGetRequest) {
	log.Printf("Starting file get for path: %s", req.Path)
	file, err := os.Open(req.Path)
	if err != nil {
		sendGetChunkError(req.TransferID, err)
		return
	}
	defer file.Close()

	buf := make([]byte, 64*1024) // 64KB chunks
	chunkIdx := 0

	for {
		n, err := file.Read(buf)
		if n > 0 {
			chunk := protocol.FileGetChunk{
				TransferID: req.TransferID,
				ChunkIndex: chunkIdx,
				Data:       base64.StdEncoding.EncodeToString(buf[:n]),
				IsEOF:      false,
			}
			chunkIdx++

			payload, _ := json.Marshal(chunk)
			msg := protocol.Message{
				Type:    protocol.TypeFileGetChunk,
				Payload: payload,
			}

			if err := writeJSONSafe(msg); err != nil {
				log.Printf("Failed to write file chunk: %v", err)
				return
			}
		}

		if err == io.EOF {
			chunk := protocol.FileGetChunk{
				TransferID: req.TransferID,
				ChunkIndex: chunkIdx,
				Data:       "",
				IsEOF:      true,
			}
			payload, _ := json.Marshal(chunk)
			msg := protocol.Message{
				Type:    protocol.TypeFileGetChunk,
				Payload: payload,
			}
			_ = writeJSONSafe(msg)
			break
		} else if err != nil {
			sendGetChunkError(req.TransferID, err)
			return
		}
	}
	log.Printf("File get completed for path: %s", req.Path)
}

func sendGetChunkError(transferID string, err error) {
	chunk := protocol.FileGetChunk{
		TransferID: transferID,
		Error:      err.Error(),
		IsEOF:      true,
	}
	payload, _ := json.Marshal(chunk)
	msg := protocol.Message{
		Type:    protocol.TypeFileGetChunk,
		Payload: payload,
	}
	_ = writeJSONSafe(msg)
}

func handleFilePut(req protocol.FilePutRequest) {
	log.Printf("Starting file put for path: %s, size: %d bytes", req.Path, req.TotalSize)
	dir := filepath.Dir(req.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %v", dir, err)
		return
	}

	file, err := os.Create(req.Path)
	if err != nil {
		log.Printf("Failed to create file %s: %v", req.Path, err)
		return
	}
	defer file.Close()

	ch := make(chan *protocol.FilePutChunk, 100)
	putTransfersMu.Lock()
	putTransfers[req.TransferID] = ch
	putTransfersMu.Unlock()

	defer func() {
		putTransfersMu.Lock()
		delete(putTransfers, req.TransferID)
		putTransfersMu.Unlock()
	}()

	for {
		select {
		case chunk := <-ch:
			if chunk.Error != "" {
				log.Printf("Received error from backend during file transfer: %s", chunk.Error)
				return
			}

			if chunk.Data != "" {
				data, err := base64.StdEncoding.DecodeString(chunk.Data)
				if err != nil {
					log.Printf("Failed to decode chunk data: %v", err)
					return
				}
				if _, err := file.Write(data); err != nil {
					log.Printf("Failed to write chunk to file: %v", err)
					return
				}
			}

			if chunk.IsEOF {
				log.Printf("File put completed for path: %s", req.Path)
				return
			}
		case <-time.After(60 * time.Second):
			log.Printf("File put timeout for transfer %s", req.TransferID)
			return
		}
	}
}

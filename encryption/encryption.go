package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gagliardetto/solana-go"
)

func EncryptFile(keyPairFile string, passphrase string) error {
	// Read the key pair JSON from the file
	file, err := os.Open(keyPairFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decode the JSON data from the file
	var keyPairData interface{}
	if err := json.NewDecoder(file).Decode(&keyPairData); err != nil {
		return err
	}

	// Marshal the keyPairData to JSON bytes
	dataToEncrypt, err := json.Marshal(keyPairData)
	if err != nil {
		return err
	}

	// Generate AES key from the passphrase (use SHA-256 to make it 32 bytes)
	key := sha256.Sum256([]byte(passphrase))

	// Create a new AES cipher block
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}

	// Generate a nonce (number used once) for GCM mode
	nonce := make([]byte, 12) // 12 bytes for GCM nonce
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	// Create the GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Encrypt the data using GCM
	ciphertext := gcm.Seal(nonce, nonce, dataToEncrypt, nil)

	// Write the encrypted data to a new file
	encFile, err := os.Create(keyPairFile + ".enc")
	if err != nil {
		return err
	}
	defer encFile.Close()

	// Base64 encode the ciphertext for readability or safe storage
	encodedCiphertext := base64.StdEncoding.EncodeToString(ciphertext)

	// Write the base64 encoded ciphertext to the file
	if _, err := encFile.Write([]byte(encodedCiphertext)); err != nil {
		return err
	}

	return nil
}
func DecryptAndLoadKeypair(encryptedFile string, passphrase string) (*solana.PrivateKey, error) {
	// Open the encrypted file
	encFile, err := os.Open(encryptedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open encrypted keypair file: %w", err)
	}
	defer encFile.Close()

	stat, err := encFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}
	// Read the entire file content
	fileSize := stat.Size()

	encodedCiphertext := make([]byte, fileSize)
	_, err = encFile.Read(encodedCiphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted data: %w", err)
	}

	// Print the size of the encoded ciphertext to help with debugging
	fmt.Println("Size of ciphertext:", len(encodedCiphertext))
	if len(encodedCiphertext) == 0 {
		return nil, fmt.Errorf("ciphertext is empty, check the file content")
	}

	// Decode the base64 encoded ciphertext
	ciphertext, err := base64.StdEncoding.DecodeString(string(encodedCiphertext))
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	// Print the size of the decoded ciphertext
	fmt.Println("Size of decoded ciphertext:", len(ciphertext))
	if len(ciphertext) < 12 {
		return nil, fmt.Errorf("ciphertext is too short, expected at least 12 bytes for nonce")
	}

	// Generate AES key from the passphrase (use SHA-256 to make it 32 bytes)
	key := sha256.Sum256([]byte(passphrase))

	// Create a new AES cipher block
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create the GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher mode: %w", err)
	}

	// Extract the nonce from the ciphertext (first 12 bytes)
	nonce, ciphertext := ciphertext[:12], ciphertext[12:]

	// Decrypt the data using GCM
	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	// Unmarshal the decrypted data into the private key bytes
	var bytes []byte
	if err := json.Unmarshal(decryptedData, &bytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted keypair: %w", err)
	}

	// Convert the bytes into a solana PrivateKey
	kp := solana.PrivateKey(bytes)

	// Return the PrivateKey
	return &kp, nil
}

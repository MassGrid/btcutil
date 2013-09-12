// Copyright (c) 2013 Conformal Systems LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcutil

import (
	"bytes"
	"fmt"
	"github.com/conformal/btcwire"
)

// OutOfRangeError describes an error due to accessing an element that is out
// of range.
type OutOfRangeError string

// BlockHeightUnknown is the value returned for a block height that is unknown.
// This is typically because the block has not been inserted into the main chain
// yet.
const BlockHeightUnknown = int64(-1)

// Error satisfies the error interface and prints human-readable errors.
func (e OutOfRangeError) Error() string {
	return string(e)
}

// Block defines a bitcoin block that provides easier and more efficient
// manipulation of raw blocks.  It also memoizes hashes for the block and its
// transactions on their first access so subsequent accesses don't have to
// repeat the relatively expensive hashing operations.
type Block struct {
	msgBlock        *btcwire.MsgBlock  // Underlying MsgBlock
	serializedBlock []byte             // Serialized bytes for the block
	blockSha        *btcwire.ShaHash   // Cached block hash
	blockHeight     int64              // Height in the main block chain
	txShas          []*btcwire.ShaHash // Cached transaction hashes
	txShasGenerated bool               // ALL transaction hashes generated
}

// MsgBlock returns the underlying btcwire.MsgBlock for the Block.
func (b *Block) MsgBlock() *btcwire.MsgBlock {
	// Return the cached block.
	return b.msgBlock
}

// Bytes returns the serialized bytes for the Block.  This is equivalent to
// calling Serialize on the underlying btcwire.MsgBlock, however it caches the
// result so subsequent calls are more efficient.
func (b *Block) Bytes() ([]byte, error) {
	// Return the cached serialized bytes if it has already been generated.
	if len(b.serializedBlock) != 0 {
		return b.serializedBlock, nil
	}

	// Serialize the MsgBlock.
	var w bytes.Buffer
	err := b.msgBlock.Serialize(&w)
	if err != nil {
		return nil, err
	}
	serializedBlock := w.Bytes()

	// Cache the serialized bytes and return them.
	b.serializedBlock = serializedBlock
	return serializedBlock, nil
}

// Sha returns the block identifier hash for the Block.  This is equivalent to
// calling BlockSha on the underlying btcwire.MsgBlock, however it caches the
// result so subsequent calls are more efficient.
func (b *Block) Sha() (*btcwire.ShaHash, error) {
	// Return the cached block hash if it has already been generated.
	if b.blockSha != nil {
		return b.blockSha, nil
	}

	// Generate the block hash.  Ignore the error since BlockSha can't
	// currently fail.
	sha, _ := b.msgBlock.BlockSha()

	// Cache the block hash and return it.
	b.blockSha = &sha
	return &sha, nil
}

// TxSha returns the hash for the requested transaction number in the Block.
// The supplied index is 0 based.  That is to say, the first transaction is the
// block is txNum 0.  This is equivalent to calling TxSha on the underlying
// btcwire.MsgTx, however it caches the result so subsequent calls are more
// efficient.
func (b *Block) TxSha(txNum int) (*btcwire.ShaHash, error) {
	// Ensure the requested transaction is in range.
	numTx := b.msgBlock.Header.TxnCount
	if txNum < 0 || uint64(txNum) > numTx {
		str := fmt.Sprintf("transaction index %d is out of range - max %d",
			txNum, numTx-1)
		return nil, OutOfRangeError(str)
	}

	// Generate slice to hold all of the transaction hashes if needed.
	if len(b.txShas) == 0 {
		b.txShas = make([]*btcwire.ShaHash, numTx)
	}

	// Return the cached hash if it has already been generated.
	if b.txShas[txNum] != nil {
		return b.txShas[txNum], nil
	}

	// Generate the hash for the transaction.  Ignore the error since TxSha
	// can't currently fail.
	sha, _ := b.msgBlock.Transactions[txNum].TxSha()

	// Cache the transaction hash and return it.
	b.txShas[txNum] = &sha
	return &sha, nil
}

// TxShas returns a slice of hashes for all transactions in the Block.  This is
// equivalent to calling TxSha on each underlying btcwire.MsgTx, however it
// caches the result so subsequent calls are more efficient.
func (b *Block) TxShas() ([]*btcwire.ShaHash, error) {
	// Return cached hashes if they have ALL already been generated.  This
	// flag is necessary because the transaction hashes are lazily generated
	// in a sparse fashion.
	if b.txShasGenerated {
		return b.txShas, nil
	}

	// Generate slice to hold all of the transaction hashes if needed.
	if len(b.txShas) == 0 {
		b.txShas = make([]*btcwire.ShaHash, b.msgBlock.Header.TxnCount)
	}

	// Generate and cache the transaction hashes for all that haven't already
	// been done.
	for i, hash := range b.txShas {
		if hash == nil {
			// Ignore the error since TxSha can't currently fail.
			sha, _ := b.msgBlock.Transactions[i].TxSha()
			b.txShas[i] = &sha
		}
	}

	b.txShasGenerated = true
	return b.txShas, nil
}

// TxLoc returns the offsets and lengths of each transaction in a raw block.
// It is used to allow fast indexing into transactions within the raw byte
// stream.
func (b *Block) TxLoc() ([]btcwire.TxLoc, error) {
	rawMsg, err := b.Bytes()
	if err != nil {
		return nil, err
	}
	rbuf := bytes.NewBuffer(rawMsg)

	var mblock btcwire.MsgBlock
	txLocs, err := mblock.DeserializeTxLoc(rbuf)
	if err != nil {
		return nil, err
	}
	return txLocs, err
}

// Height returns the saved height of the block in the block chain.  This value
// will be BlockHeightUnknown if it hasn't already explicitly been set.
func (b *Block) Height() int64 {
	return b.blockHeight
}

// SetHeight sets the height of the block in the block chain.
func (b *Block) SetHeight(height int64) {
	b.blockHeight = height
}

// NewBlock returns a new instance of a bitcoin block given an underlying
// btcwire.MsgBlock.  See Block.
func NewBlock(msgBlock *btcwire.MsgBlock) *Block {
	return &Block{
		msgBlock:    msgBlock,
		blockHeight: BlockHeightUnknown,
	}
}

// NewBlockFromBytes returns a new instance of a bitcoin block given the
// serialized bytes.  See Block.
func NewBlockFromBytes(serializedBlock []byte) (*Block, error) {
	// Deserialize the bytes into a MsgBlock.
	var msgBlock btcwire.MsgBlock
	br := bytes.NewBuffer(serializedBlock)
	err := msgBlock.Deserialize(br)
	if err != nil {
		return nil, err
	}

	b := Block{
		msgBlock:        &msgBlock,
		serializedBlock: serializedBlock,
		blockHeight:     BlockHeightUnknown,
	}
	return &b, nil
}

// NewBlockFromBlockAndBytes returns a new instance of a bitcoin block given
// an underlying btcwire.MsgBlock and the serialized bytes for it.  See Block.
func NewBlockFromBlockAndBytes(msgBlock *btcwire.MsgBlock, serializedBlock []byte) *Block {
	return &Block{
		msgBlock:        msgBlock,
		serializedBlock: serializedBlock,
		blockHeight:     BlockHeightUnknown,
	}
}
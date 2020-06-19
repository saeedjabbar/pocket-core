package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	appsTypes "github.com/pokt-network/pocket-core/x/apps/types"
	"github.com/pokt-network/pocket-core/x/pocketcore/types"
	sdk "github.com/pokt-network/posmint/types"
	"github.com/stretchr/testify/assert"
	"time"
)

func TestKeeper_ValidateProof(t *testing.T) { // happy path only todo
	ctx, _, _, _, keeper, keys, _ := createTestInput(t, false)
	types.ClearEvidence()
	npk, header, _, _ := simulateRelays(t, keeper, &ctx, 5)
	evidence, err := types.GetEvidence(header, types.RelayEvidence, sdk.NewInt(1000))
	if err != nil {
		t.Fatalf("Set evidence not found")
	}
	root := evidence.GenerateMerkleRoot()
	_, totalRelays := types.GetTotalProofs(header, types.RelayEvidence, sdk.NewInt(1000))
	assert.Equal(t, totalRelays, int64(5))
	// generate a claim message
	claimMsg := types.MsgClaim{
		SessionHeader: header,
		MerkleRoot:    root,
		TotalProofs:   5,
		FromAddress:   sdk.Address(npk.Address()),
		EvidenceType:  types.RelayEvidence,
	}
	mockCtx := &Ctx{}
	mockCtx.On("KVStore", keeper.storeKey).Return(ctx.KVStore(keeper.storeKey))
	mockCtx.On("KVStore", keys[sdk.ParamsKey.Name()]).Return(ctx.KVStore(keys[sdk.ParamsKey.Name()]))
	mockCtx.On("KVStore", keys[appsTypes.StoreKey]).Return(ctx.KVStore(keys[appsTypes.StoreKey]))
	mockCtx.On("Logger").Return(ctx.Logger())
	mockCtx.On("BlockHeight").Return(ctx.BlockHeight())
	mockCtx.On("PrevCtx", header.SessionBlockHeight).Return(ctx, nil)
	mockCtx.On("PrevCtx", header.SessionBlockHeight+keeper.ClaimSubmissionWindow(ctx)*keeper.BlocksPerSession(ctx)).Return(ctx, nil)

	// generate the pseudorandom proof
	neededLeafIndex, er := keeper.getPseudorandomIndex(mockCtx, totalRelays, header, mockCtx)
	assert.Nil(t, er)
	merkleProofs, _ := evidence.GenerateMerkleProof(int(neededLeafIndex))
	// get leaf and cousin node
	leafNode := types.GetProof(header, types.RelayEvidence, neededLeafIndex)
	// create proof message
	proofMsg := types.MsgProof{
		MerkleProof:  merkleProofs,
		Leaf:         leafNode.(types.RelayProof),
		EvidenceType: types.RelayEvidence,
	}
	err = keeper.SetClaim(mockCtx, claimMsg)
	if err != nil {
		t.Fatal(err)
	}
	// validate proof
	_, _, err = keeper.ValidateProof(mockCtx, proofMsg)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestKeeper_GetPsuedorandomIndex(t *testing.T) {
	var totalRelays []int = []int{10, 100, 10000000}
	for _, relays := range totalRelays {
		ctx, _, _, _, keeper, keys, _ := createTestInput(t, false)
		header := types.SessionHeader{
			ApplicationPubKey:  "asdlfj",
			Chain:              "lkajsdf",
			SessionBlockHeight: 1,
		}
		mockCtx := new(Ctx)
		mockCtx.On("KVStore", keeper.storeKey).Return(ctx.KVStore(keeper.storeKey))
		mockCtx.On("KVStore", keys["params"]).Return(ctx.KVStore(keys["params"]))
		mockCtx.On("PrevCtx", header.SessionBlockHeight+keeper.ClaimSubmissionWindow(ctx)*keeper.BlocksPerSession(ctx)).Return(ctx, nil)

		// generate the pseudorandom proof
		neededLeafIndex, err := keeper.getPseudorandomIndex(mockCtx, int64(relays), header, mockCtx)
		assert.Nil(t, err)
		assert.LessOrEqual(t, neededLeafIndex, int64(relays))
	}
}

func TestKeeper_GetSetReceipt(t *testing.T) {
	ctx, _, _, _, keeper, _, _ := createTestInput(t, false)
	appPrivateKey := getRandomPrivateKey()
	appPubKey := appPrivateKey.PublicKey().RawString()
	npk := getRandomPubKey()
	ethereum := hex.EncodeToString([]byte{01})
	// create a session header
	validHeader := types.SessionHeader{
		ApplicationPubKey:  appPubKey,
		Chain:              ethereum,
		SessionBlockHeight: 976,
	}
	receipt := types.Receipt{
		SessionHeader:   validHeader,
		ServicerAddress: sdk.Address(npk.Address()).String(),
		Total:           2000,
		EvidenceType:    types.RelayEvidence,
	}
	addr := sdk.Address(sdk.Address(npk.Address()))
	mockCtx := new(Ctx)
	mockCtx.On("KVStore", keeper.storeKey).Return(ctx.KVStore(keeper.storeKey))
	mockCtx.On("PrevCtx", validHeader.SessionBlockHeight).Return(ctx, nil)
	mockCtx.On("Logger").Return(ctx.Logger())
	_ = keeper.SetReceipt(mockCtx, addr, receipt)

	inv, found := keeper.GetReceipt(mockCtx, sdk.Address(npk.Address()), validHeader, receipt.EvidenceType)
	assert.True(t, found)
	assert.Equal(t, inv, receipt)
}

func TestKeeper_GetSetReceipts(t *testing.T) {
	ctx, _, _, _, keeper, _, _ := createTestInput(t, false)
	appPrivateKey := getRandomPrivateKey()
	appPubKey := appPrivateKey.PublicKey().RawString()
	appPrivateKey2 := getRandomPrivateKey()
	appPubKey2 := appPrivateKey2.PublicKey().RawString()
	npk := getRandomPubKey()
	ethereum := hex.EncodeToString([]byte{01})
	// create a session header
	validHeader := types.SessionHeader{
		ApplicationPubKey:  appPubKey,
		Chain:              ethereum,
		SessionBlockHeight: 1,
	}
	// create a session header
	validHeader2 := types.SessionHeader{
		ApplicationPubKey:  appPubKey2,
		Chain:              ethereum,
		SessionBlockHeight: 1,
	}
	receipt := types.Receipt{
		SessionHeader:   validHeader,
		ServicerAddress: sdk.Address(npk.Address()).String(),
		Total:           2000,
		EvidenceType:    types.RelayEvidence,
	}
	receipt2 := types.Receipt{
		SessionHeader:   validHeader2,
		ServicerAddress: sdk.Address(npk.Address()).String(),
		Total:           2000,
		EvidenceType:    types.RelayEvidence,
	}
	receipts := []types.Receipt{receipt, receipt2}
	mockCtx := new(Ctx)
	mockCtx.On("KVStore", keeper.storeKey).Return(ctx.KVStore(keeper.storeKey))
	mockCtx.On("PrevCtx", validHeader.SessionBlockHeight).Return(ctx, nil)
	mockCtx.On("Logger").Return(ctx.Logger())
	keeper.SetReceipts(mockCtx, receipts)
	inv, er := keeper.GetReceipts(mockCtx, sdk.Address(sdk.Address(npk.Address())))
	assert.Nil(t, er)
	assert.Contains(t, inv, receipt)
	assert.Contains(t, inv, receipt2)
}

func TestKeeper_GetAllReceipts(t *testing.T) {
	ctx, _, _, _, keeper, _, _ := createTestInput(t, false)
	appPrivateKey := getRandomPrivateKey()
	appPubKey := appPrivateKey.PublicKey().RawString()
	npk := getRandomPubKey()
	npk2 := getRandomPubKey()
	ethereum := hex.EncodeToString([]byte{01})
	// create a session header
	validHeader := types.SessionHeader{
		ApplicationPubKey:  appPubKey,
		Chain:              ethereum,
		SessionBlockHeight: 1,
	}
	receipt := types.Receipt{
		SessionHeader:   validHeader,
		ServicerAddress: sdk.Address(npk.Address()).String(),
		Total:           2000,
		EvidenceType:    types.RelayEvidence,
	}
	receipt2 := types.Receipt{
		SessionHeader:   validHeader,
		ServicerAddress: sdk.Address(npk2.Address()).String(),
		Total:           2000,
		EvidenceType:    types.RelayEvidence,
	}
	receipts := []types.Receipt{receipt, receipt2}
	mockCtx := new(Ctx)
	mockCtx.On("KVStore", keeper.storeKey).Return(ctx.KVStore(keeper.storeKey))
	mockCtx.On("PrevCtx", validHeader.SessionBlockHeight).Return(ctx, nil)
	mockCtx.On("Logger").Return(ctx.Logger())
	keeper.SetReceipts(mockCtx, receipts)
	inv := keeper.GetAllReceipts(mockCtx)
	assert.Contains(t, inv, receipt)
	assert.Contains(t, inv, receipt2)
}

func TestPseudoRandomSelection(t *testing.T) {
	// maximum index selection
	const max = uint64(1000)
	const iterations = 10000
	// an index account array for proof
	dataArr := make([]int64, max)
	// run a for loop for statistics
	for i := 0; i < iterations; i++ {
		// create random seed data
		seed := make([]byte, 8)
		binary.LittleEndian.PutUint64(seed, rand.New(rand.NewSource(time.Now().UnixNano())).Uint64())
		// hash for show and convert back to decimal
		blockHashDecimal := binary.LittleEndian.Uint64(types.Hash(seed))
		// mod the selection
		selection := blockHashDecimal % max
		// increment the data
		dataArr[selection] = dataArr[selection] + 1
	}
	// print the results
	for i := 0; uint64(i) < max; i++ {
		fmt.Printf("index %d, was selected %d times\n", i, dataArr[i])
	}
}

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/babylonlabs-io/babylon/test/e2e/util"
	btcstakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
	codetypese "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/gogoproto/proto"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ExtractResult struct {
	Counts string
	Txs    map[string]string
}

const (
	TOTAL                     = "total"
	MSG_ADD_COVENANT_SIGS     = "/babylon.btcstaking.v1.MsgAddCovenantSigs"
	MSG_ADD_FINALITY_SIG      = "/babylon.finality.v1.MsgAddFinalitySig"
	MSG_CREATE_BTC_DELEGATION = "/babylon.btcstaking.v1.MsgCreateBTCDelegation"

	RPC_URL = "https://babylon-testnet-rpc.polkachu.com/block"
	OUT_DIR = "out"
)

func main() {
	if len(os.Args) < 2 {
		panic("you should set file path to read")
	}

	hc := &http.Client{
		Timeout: 10 * time.Second,
	}

	var (
		eg      errgroup.Group
		results = make([]ExtractResult, len(os.Args[1:]))
	)

	if _, err := os.Stat(OUT_DIR); os.IsNotExist(err) {
		err = os.Mkdir(OUT_DIR, 0755)
		if err != nil {
			panic(err)
		}
	}

	for i, heightStr := range os.Args[1:] {
		i, heightStr := i, heightStr // avoid closure issue
		eg.Go(func() error {
			result, err := extract(hc, heightStr)
			if err != nil {
				return fmt.Errorf("failed to extract block %s: %v", heightStr, err)
			}
			results[i] = *result
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := eg.Wait(); err != nil {
		panic(err)
	}

	// Sort results by block height
	sort.SliceStable(results, func(i, j int) bool {
		return extractHeight(results[i].Counts) < extractHeight(results[j].Counts)
	})

	var aggregation []byte
	// Write results to files
	for i, result := range results {
		heightStr := os.Args[1:][i]
		countsFileName := fmt.Sprintf("%s.count", heightStr)
		if err := os.WriteFile(path.Join(OUT_DIR, countsFileName), []byte(result.Counts), 0644); err != nil {
			fmt.Printf("Failed to write to file %s: %v\n", countsFileName, err)
		}

		for k, tx := range result.Txs {
			txsFileName := fmt.Sprintf("%s.%s.txs.json", heightStr, strings.Split(k, ".")[len(strings.Split(k, "."))-1])
			if err := os.WriteFile(path.Join(OUT_DIR, txsFileName), []byte(tx), 0644); err != nil {
				fmt.Printf("Failed to write to file %s: %v\n", txsFileName, err)
			}
		}

		aggregation = append(aggregation, []byte(result.Counts+"\n")...)
	}
	err := os.WriteFile(path.Join(OUT_DIR, "aggregation.txt"), aggregation, 0644)
	if err != nil {
		panic(err)
	}
	println(string(aggregation))
}

func extract(hc *http.Client, heightStr string) (*ExtractResult, error) {
	var (
		height        uint64
		err           error
		res           *http.Response
		b             []byte
		blockFileName = path.Join(OUT_DIR, fmt.Sprintf("%s.block", heightStr))
	)

	if _, err = os.Stat(blockFileName); os.IsNotExist(err) {
		height, err = strconv.ParseUint(heightStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse height: %v", err)
		}

		res, err = hc.Get(fmt.Sprintf("%s?height=%d", RPC_URL, height))
		if err != nil {
			return nil, fmt.Errorf("failed to query block: %v", err)
		}
		defer res.Body.Close()

		b, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read block: %v", err)
		}

		if err := os.WriteFile(blockFileName, b, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to write block file: %v", err)
		}
	} else {
		b, err = os.ReadFile(blockFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to read block file: %v", err)
		}
	}

	var block Block
	err = json.Unmarshal(b, &block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	var (
		output      bytes.Buffer
		msgMap      = make(map[string]uint)
		resultJsons = make(map[string][]byte)
	)

	for _, rawTx := range block.Result.Block.Data.Txs {
		var (
			txBytes      []byte
			tx           *sdktx.Tx
			marshaledMsg []byte
		)
		txBytes, err = base64.StdEncoding.DecodeString(rawTx)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tx: %v", err)
		}

		tx, err = decodeTx(txBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tx: %v", err)
		}

		for _, rawMsg := range tx.GetBody().GetMessages() {
			msgMap[TOTAL]++
			switch rawMsg.TypeUrl {
			case MSG_ADD_COVENANT_SIGS:
				resMsg, _ := parse(rawMsg, &btcstakingtypes.MsgAddCovenantSigs{})
				marshaledMsg, err = json.MarshalIndent((*resMsg).(*btcstakingtypes.MsgAddCovenantSigs), "", "    ")
			case MSG_ADD_FINALITY_SIG:
				resMsg, _ := parse(rawMsg, &finalitytypes.MsgAddFinalitySig{})
				marshaledMsg, err = json.MarshalIndent((*resMsg).(*finalitytypes.MsgAddFinalitySig), "", "    ")
			case MSG_CREATE_BTC_DELEGATION:
				resMsg, _ := parse(rawMsg, &btcstakingtypes.MsgCreateBTCDelegation{})
				marshaledMsg, err = json.MarshalIndent((*resMsg).(*btcstakingtypes.MsgCreateBTCDelegation), "", "    ")
			default:
			}
			if err != nil {
				return nil, fmt.Errorf("failed to marshal resMsg: %v", err)
			}

			if marshaledMsg != nil {
				marshaledMsg = append(marshaledMsg, []byte(",\n")...)

				if len(resultJsons[rawMsg.TypeUrl]) == 0 {
					resultJsons[rawMsg.TypeUrl] = []byte("[\n")
				}
				resultJsons[rawMsg.TypeUrl] = append(resultJsons[rawMsg.TypeUrl], marshaledMsg...)
			}
			msgMap[rawMsg.TypeUrl]++
		}

	}

	keys := make([]string, 0, len(msgMap))
	for k := range msgMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	output.WriteString(fmt.Sprintf("%s counts\n", heightStr))
	output.WriteString(fmt.Sprintf("%s: %d\n", TOTAL, msgMap[TOTAL]))
	for _, k := range keys {
		if k == TOTAL {
			continue
		}
		output.WriteString(fmt.Sprintf("%s: %d\n", k, msgMap[k]))
	}
	output.WriteString("=============\n")

	var resultJsonsMap = make(map[string]string)

	for k, j := range resultJsons {
		resultJsonsMap[k] = string(append(j[:len(j)-2], []byte("\n]")...))
	}

	return &ExtractResult{
		Counts: output.String(),
		Txs:    resultJsonsMap,
	}, nil
}

func extractHeight(result string) uint64 {
	var height uint64
	fmt.Sscanf(result, "%d counts", &height)
	return height
}

func parse(rawMsg *codetypese.Any, t proto.Message) (*proto.Message, error) {
	msg, ok := t.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("provided type does not implement proto.Message")
	}

	err := proto.Unmarshal(rawMsg.Value, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

func decodeTx(txBytes []byte) (*sdktx.Tx, error) {
	var raw sdktx.TxRaw

	// reject all unknown proto fields in the root TxRaw
	err := unknownproto.RejectUnknownFieldsStrict(txBytes, &raw, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.Cdc.Unmarshal(txBytes, &raw); err != nil {
		return nil, err
	}

	var body sdktx.TxBody
	if err := util.Cdc.Unmarshal(raw.BodyBytes, &body); err != nil {
		return nil, fmt.Errorf("failed to decode tx: %w", err)
	}

	var authInfo sdktx.AuthInfo

	// reject all unknown proto fields in AuthInfo
	err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.Cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
		return nil, fmt.Errorf("failed to decode auth info: %w", err)
	}

	return &sdktx.Tx{
		Body:       &body,
		AuthInfo:   &authInfo,
		Signatures: raw.Signatures,
	}, nil
}

type Block struct {
	Result struct {
		Block struct {
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
			Evidence struct {
				Evidence []interface{} `json:"evidence"`
			} `json:"evidence"`
			LastCommit struct {
				Round      int64       `json:"round"`
				BlockID    BlockID     `json:"block_id"`
				Signatures []Signature `json:"signatures"`
				Height     string      `json:"height"`
			} `json:"last_commit"`
			Header struct {
				ValidatorsHash     string `json:"validators_hash"`
				ChainID            string `json:"chain_id"`
				ConsensusHash      string `json:"consensus_hash"`
				ProposerAddress    string `json:"proposer_address"`
				NextValidatorsHash string `json:"next_validators_hash"`
				Version            struct {
					Block string `json:"block"`
				} `json:"version"`
				DataHash        string  `json:"data_hash"`
				LastResultsHash string  `json:"last_results_hash"`
				LastBlockID     BlockID `json:"last_block_id"`
				EvidenceHash    string  `json:"evidence_hash"`
				AppHash         string  `json:"app_hash"`
				Time            string  `json:"time"`
				Height          string  `json:"height"`
				LastCommitHash  string  `json:"last_commit_hash"`
			} `json:"header"`
		} `json:"block"`
		BlockID BlockID `json:"block_id"`
	} `json:"result"`
	ID      int64  `json:"id"`
	Jsonrpc string `json:"jsonrpc"`
}

type BlockID struct {
	Parts struct {
		Total int64  `json:"total"`
		Hash  string `json:"hash"`
	} `json:"parts"`
	Hash string `json:"hash"`
}

type Signature struct {
	Signature        *string `json:"signature,omitempty"`
	ValidatorAddress string  `json:"validator_address"`
	BlockIDFlag      int64   `json:"block_id_flag"`
	Timestamp        string  `json:"timestamp"`
}

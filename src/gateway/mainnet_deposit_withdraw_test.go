package gateway

import (
	"client"
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/loomnetwork/go-loom"
	loom_client "github.com/loomnetwork/go-loom/client"
	"github.com/stretchr/testify/suite"

	// Contract bindings
	tgtypes "github.com/loomnetwork/go-loom/builtin/types/transfer_gateway"
	am "github.com/loomnetwork/go-loom/client/address_mapper"
	"github.com/loomnetwork/go-loom/client/erc20"
	"github.com/loomnetwork/go-loom/client/erc721"
	"github.com/loomnetwork/go-loom/client/erc721x"
	"github.com/loomnetwork/go-loom/client/evm_eth"
	gw "github.com/loomnetwork/go-loom/client/gateway_v2"
	"github.com/loomnetwork/go-loom/client/native_coin"
	vmc "github.com/loomnetwork/go-loom/client/validator_manager"
)

type TransferGatewayTestSuite struct {
	suite.Suite
	oracleWaitTime time.Duration
	ethRPCClient   *rpc.Client
	ethClient      *ethclient.Client
	loomClient     *loom_client.DAppChainRPCClient

	// Contracts
	addressMapper                *am.DAppChainAddressMapper
	dappchainGateway             *gw.DAppChainGateway
	dappchainLoomGateway         *gw.DAppChainGateway
	validatorsManager            *vmc.MainnetVMCClient
	mainnetGateway               *gw.MainnetGatewayClient
	mainnetLoomGateway           *gw.MainnetGatewayClient
	mainnetCards                 *client.MainnetCryptoCardsClient
	mainnetERC721                *client.MainnetERC721MintableContract
	mainnetERC721X               *client.MainnetERC721XContract
	mainnetCoin                  *client.MainnetERC20Contract
	mainnetCoin2                 *client.MainnetERC20MintableContract
	mainnetLoomCoin              *client.MainnetERC20Contract
	loomERC721X                  *erc721x.DAppChainERC721XContract
	loomERC721                   *erc721.DAppChainERC721Contract
	loomERC721_2                 *erc721.DAppChainERC721Contract
	loomERC20                    *erc20.DAppChainERC20Contract
	loomERC20_2                  *erc20.DAppChainERC20Contract
	loomCoin                     *native_coin.DAppChainNativeCoin
	loomEth                      *native_coin.DAppChainNativeCoin
	onGanache                    bool
	numMainnetBlockConfirmations int

	// These identities are shared by all the tests
	gatewayCreator *loom_client.Identity
	cardsCreator   *loom_client.Identity
	coinCreator    *loom_client.Identity
	alice          *loom_client.Identity
	bob            *loom_client.Identity
}

func TestTransferGatewayTestSuite(t *testing.T) {
	suite.Run(t, new(TransferGatewayTestSuite))
}

// Both Loom and Ganache must be running before starting this test suite.
// LOOM_DIR env var should point to the directory containing loom.yml.
// ORACLE_RUNNING=true will make tests wait for 5 seconds each time the Oracle is expected to do
// something, leaving this env var unset will make the test trigger the Oracle directly.
// ORACLE_WAIT_TIME env var can be set to adjust how many seconds the test should wait for the
// Oracle to do its job.
// loom_e2e_tests.sh script will set everything up and then execute the tests.
func (s *TransferGatewayTestSuite) SetupSuite() {
	require := s.Require()
	require.NotEmpty(os.Getenv("LOOM_DIR"), "LOOM_DIR env var should be set to dir containing loom.yml")
	var err error
	s.oracleWaitTime = time.Duration(10) * time.Second
	if os.Getenv("ORACLE_WAIT_TIME") != "" {
		secs, err := strconv.ParseInt(os.Getenv("ORACLE_WAIT_TIME"), 10, 32)
		require.NoError(err)
		s.oracleWaitTime = time.Duration(secs) * time.Second
	}

	ethNet := os.Getenv("ETHEREUM_NETWORK")
	if ethNet == "" || ethNet == "ganache" {
		s.onGanache = true
	}

	loomCfg, err := ParseConfig([]string{os.Getenv("LOOM_DIR")})
	require.NoError(err)

	s.ethRPCClient, err = rpc.DialContext(context.Background(), loomCfg.TransferGateway.EthereumURI)
	require.NoError(err)
	s.ethClient = ethclient.NewClient(s.ethRPCClient)

	fmt.Println(loomCfg.ChainID, loomCfg.TransferGateway.DAppChainReadURI, loomCfg.TransferGateway.DAppChainWriteURI)

	s.loomClient = loom_client.NewDAppChainRPCClient(
		loomCfg.ChainID,
		loomCfg.TransferGateway.DAppChainWriteURI,
		loomCfg.TransferGateway.DAppChainReadURI,
	)

	s.numMainnetBlockConfirmations = loomCfg.TransferGateway.NumMainnetBlockConfirmations

	// Connect dappchain contracts

	s.addressMapper, err = am.ConnectToDAppChainAddressMapper(s.loomClient)
	require.NoError(err)

	s.dappchainGateway, err = gw.ConnectToDAppChainGateway(s.loomClient, loomCfg.TransferGateway.DAppChainEventsURI)
	require.NoError(err)

	s.loomCoin, err = native_coin.ConnectToDAppChainLoomContract(s.loomClient)
	require.NoError(err)

	s.loomEth, err = native_coin.ConnectToDAppChainETHContract(s.loomClient)
	require.NoError(err)

	// erc20 token
	dapptokenaddr := GetMainnetContractCfgString("loomchain_SampleERC20Token_1")
	mirroredErc20TokenContract, err := ConnectToTokenContractByAddress(s.loomClient, "../ethcontract/SampleERC20Token.abi",
		"SampleERC20Token", loom.MustParseAddress("default:"+dapptokenaddr))
	require.NoError(err)
	s.loomERC20 = &erc20.DAppChainERC20Contract{MirroredTokenContract: mirroredErc20TokenContract}
	require.NoError(err)

	// new mintable token
	dapptokenaddr = GetMainnetContractCfgString("loomchain_SampleERC20Token_2")
	mirroredErc20TokenContract2, err := ConnectToTokenContractByAddress(s.loomClient, "../ethcontract/SampleERC20Token.abi",
		"SampleERC20Token", loom.MustParseAddress("default:"+dapptokenaddr))
	require.NoError(err)
	s.loomERC20_2 = &erc20.DAppChainERC20Contract{MirroredTokenContract: mirroredErc20TokenContract2}
	require.NoError(err)

	dappeErc721tokenaddr := GetMainnetContractCfgString("loomchain_crypto_cards_addr")
	mirroredErc721TokenContract, err := ConnectToTokenContractByAddress(s.loomClient, "../ethcontract/SampleERC721Token.abi",
		"SampleERC721Token", loom.MustParseAddress("default:"+dappeErc721tokenaddr))
	require.NoError(err)
	s.loomERC721 = &erc721.DAppChainERC721Contract{MirroredTokenContract: mirroredErc721TokenContract}
	require.NoError(err)

	dappeErc721xTokenaddr := GetMainnetContractCfgString("loomchain_SampleERC721XToken_1")
	mirroredErc721XTokenContract, err := ConnectToTokenContractByAddress(s.loomClient, "../ethcontract/SampleERC721XToken.abi",
		"SampleERC721XToken", loom.MustParseAddress("default:"+dappeErc721xTokenaddr))
	require.NoError(err)
	s.loomERC721X = &erc721x.DAppChainERC721XContract{MirroredTokenContract: mirroredErc721XTokenContract}
	require.NoError(err)

	dappeErc721mintableTokenaddr := GetMainnetContractCfgString("loomchain_erc721_mintable_token_addr")
	mirroredErc721mintableTokenContract, err := ConnectToTokenContractByAddress(s.loomClient, "../ethcontract/SampleERC721Token.abi",
		"SampleERC721Token", loom.MustParseAddress("default:"+dappeErc721mintableTokenaddr))
	require.NoError(err)
	s.loomERC721_2 = &erc721.DAppChainERC721Contract{MirroredTokenContract: mirroredErc721mintableTokenContract}
	require.NoError(err)

	// Connect mainnet contracts

	vmcAddr := GetMainnetContractCfgString("mainnet_validatormanagercontract_addr")
	s.validatorsManager, err = vmc.ConnectToMainnetVMCClient(s.ethClient, vmcAddr)
	require.NoError(err)

	mainnetGatewayAddr := GetMainnetContractCfgString("mainnet_gateway_addr")
	s.mainnetGateway, err = gw.ConnectToMainnetGateway(s.ethClient, mainnetGatewayAddr)
	require.NoError(err)

	mainnetLoomGatewayAddr := GetMainnetContractCfgString("mainnet_loomgateway_addr")
	s.mainnetLoomGateway, err = gw.ConnectToMainnetGateway(s.ethClient, mainnetLoomGatewayAddr)
	require.NoError(err)

	erc721Addr := GetMainnetContractCfgString("mainnet_crypto_cards_addr")
	s.mainnetCards, err = client.ConnectToMainnetCards(s.ethClient, erc721Addr)
	require.NoError(err)

	erc721Addr2 := GetMainnetContractCfgString("mainnet_erc721_mintable_token_addr")
	s.mainnetERC721, err = client.ConnectToMainnetERC721MintableContract(s.ethClient, erc721Addr2)
	require.NoError(err)

	erc721XAddr := GetMainnetContractCfgString("mainnet_erc721x_cards_addr")
	s.mainnetERC721X, err = client.ConnectToMainnetERC721XContract(s.ethClient, erc721XAddr)
	require.NoError(err)

	erc20Addr := GetMainnetContractCfgString("mainnet_game_token_addr")
	s.mainnetCoin, err = client.ConnectToMainnetERC20Contract(s.ethClient, erc20Addr)
	require.NoError(err)

	erc20Addr2 := GetMainnetContractCfgString("mainnet_erc20_mintable_token_addr")
	s.mainnetCoin2, err = client.ConnectToMainnetERC20MintableContract(s.ethClient, erc20Addr2)
	require.NoError(err)

	loomAddr := GetMainnetContractCfgString("loomtoken_addr")
	s.mainnetLoomCoin, err = client.ConnectToMainnetERC20Contract(s.ethClient, loomAddr)
	require.NoError(err)

	// Create identities

	ethKey, dappchainKey := GetKeys("trudy")
	s.gatewayCreator, err = loom_client.CreateIdentityStr(ethKey, dappchainKey, s.loomClient.GetChainID())
	require.NoError(err)

	ethKey, dappchainKey = GetKeys("dan")
	s.cardsCreator, err = loom_client.CreateIdentityStr(ethKey, dappchainKey, s.loomClient.GetChainID())
	require.NoError(err)

	ethKey, dappchainKey = GetKeys("trudy")
	s.coinCreator, err = loom_client.CreateIdentityStr(ethKey, dappchainKey, s.loomClient.GetChainID())
	require.NoError(err)

	ethKey, dappchainKey = GetKeys("alice")
	s.alice, err = loom_client.CreateIdentityStr(ethKey, dappchainKey, s.loomClient.GetChainID())
	require.NoError(err)

	ethKey, dappchainKey = GetKeys("bob")
	s.bob, err = loom_client.CreateIdentityStr(ethKey, dappchainKey, s.loomClient.GetChainID())
	require.NoError(err)

	fmt.Println(time.Now().UTC())
	// Associate Alice's Mainnet account with her DAppChain account (only if mapping doesn't exist already, helps us run multiple e2e tests sequentially if needed)
	exists, _ := s.addressMapper.HasIdentityMapping(s.alice.LoomAddr)
	if !exists {
		require.NoError(s.addressMapper.AddIdentityMapping(s.alice))
	}

	time.Sleep(10 * time.Second)
}

func (s *TransferGatewayTestSuite) mineBlock() {
	// evm_mine is only implemented by Ganache
	if !s.onGanache {
		return
	}
	s.Require().NoError(s.ethRPCClient.CallContext(context.TODO(), nil, "evm_mine"))
}

func (s *TransferGatewayTestSuite) mineBlocksTillConfirmation() {
	for i := 0; i < s.numMainnetBlockConfirmations+1; i++ {
		s.mineBlock()
	}
}

func (s *TransferGatewayTestSuite) TestERC721DepositAndWithdraw() {
	var err error
	require := s.Require()
	alice := s.alice

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Give Alice some ERC721 tokens on Mainnet
	require.NoError(s.mainnetCards.MintTokens(s.cardsCreator, alice))
	aliceMainnetCardStartBal, err := s.mainnetCards.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits one of her tokens to the Mainnet Gateway contract
	aliceTokenID, err := s.mainnetCards.TokenOfOwnerByIndex(alice, 0)
	require.NoError(err)
	require.NoError(s.mainnetCards.DepositToGateway(alice, aliceTokenID))
	curBalance, err := s.mainnetCards.BalanceOf(alice)
	require.NoError(err)
	require.Equal(aliceMainnetCardStartBal-1, curBalance)
	isTokenDeposited, err := s.mainnetGateway.ERC721Deposited(aliceTokenID, s.mainnetCards.Address)
	require.NoError(err)
	require.True(isTokenDeposited, "Alice's token should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her token in the DAppChain ERC721 contract
	ownerAddr, err := s.loomERC721.OwnerOf(aliceTokenID)
	require.NoError(err)
	require.Equal(alice.LoomAddr.Local.Hex(), ownerAddr.Hex())

	// Alice must grant approval to the DAppChain Gateway to take ownership of the token when it's withdrawn
	require.NoError(s.loomERC721.Approve(alice, s.dappchainGateway.Address, aliceTokenID))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC721(alice, aliceTokenID, s.loomERC721.Address, nil)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// and receives a withdrawal receipt...
	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	// Alice can now withdraw the token from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetGateway.WithdrawERC721(alice, aliceTokenID, s.mainnetCards.Address, wr.OracleSignature, validators))

	// Alice should now have her token back on Mainnet
	isTokenDeposited, err = s.mainnetGateway.ERC721Deposited(aliceTokenID, s.mainnetCards.Address)
	require.NoError(err)
	require.False(isTokenDeposited, "Alice's token shouldn't be deposited in the Mainnet Gateway")
	aliceEndBalance, err := s.mainnetCards.BalanceOf(alice)
	require.NoError(err)
	require.Equal(aliceMainnetCardStartBal, aliceEndBalance)

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

// Alice transfers an ERC721 token from Mainnet to DAppChain, then transfers it to Bob on the
// DAppChain, who then withdraws the token from the DAppChain to Mainnet.
func (s *TransferGatewayTestSuite) TestERC721DepositTransferWithdraw() {
	var err error
	require := s.Require()

	alice := s.alice
	bob := s.bob

	require.NoError(err)

	// Give Alice some ERC721 tokens on Mainnet
	require.NoError(s.mainnetCards.MintTokens(s.cardsCreator, alice))
	aliceMainnetCardStartBal, err := s.mainnetCards.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits one of her tokens to the Mainnet Gateway contract
	aliceTokenID, err := s.mainnetCards.TokenOfOwnerByIndex(alice, 0)
	require.NoError(err)
	require.NoError(s.mainnetCards.DepositToGateway(alice, aliceTokenID))
	curBalance, err := s.mainnetCards.BalanceOf(alice)
	require.NoError(err)
	require.Equal(aliceMainnetCardStartBal-1, curBalance)
	isTokenDeposited, err := s.mainnetGateway.ERC721Deposited(aliceTokenID, s.mainnetCards.Address)
	require.NoError(err)
	require.True(isTokenDeposited, "Alice's token should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her token in the DAppChain ERC721 contract
	ownerAddr, err := s.loomERC721.OwnerOf(aliceTokenID)
	require.NoError(err)
	require.Equal(alice.LoomAddr.Local.Hex(), ownerAddr.Hex())

	// Alice transfers her token to Bob
	require.NoError(s.loomERC721.TransferFrom(alice, bob, aliceTokenID))

	// Bob must grant approval to the DAppChain Gateway to take ownership of the token when it's withdrawn
	require.NoError(s.loomERC721.Approve(bob, s.dappchainGateway.Address, aliceTokenID))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(bob)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Bob can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC721(bob, aliceTokenID, s.loomERC721.Address, &bob.MainnetAddr)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// and receives a withdrawal receipt...
	wr, err := s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.NotNil(wr)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	// Alice can now withdraw the token from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetGateway.WithdrawERC721(bob, aliceTokenID, s.mainnetCards.Address, wr.OracleSignature, validators))

	// Bob should now have the token Alice sent him, in his Mainnet account
	isTokenDeposited, err = s.mainnetGateway.ERC721Deposited(aliceTokenID, s.mainnetCards.Address)
	require.NoError(err)
	require.False(isTokenDeposited, "Bob's token shouldn't be deposited in the Mainnet Gateway")
	tokenOwner, err := s.mainnetCards.OwnerOf(aliceTokenID)
	require.NoError(err)
	require.Equal(bob.MainnetAddr.String(), tokenOwner.String(), "Bob should own Alice's token on Mainnet")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Bob's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestERC721MintableWithdraw() {
	var err error
	require := s.Require()
	alice := s.alice
	aliceTokenID := big.NewInt(123)

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Give Alice some ERC721 tokens on Dappchain
	require.NoError(s.loomERC721_2.MintTo(s.cardsCreator, alice.LoomAddr, aliceTokenID))

	// Alice should now have her token in the DAppChain ERC721 contract
	ownerAddr, err := s.loomERC721_2.OwnerOf(aliceTokenID)
	require.NoError(err)
	require.Equal(alice.LoomAddr.Local.Hex(), ownerAddr.Hex())

	// Alice must grant approval to the DAppChain Gateway to take ownership of the token when it's withdrawn
	require.NoError(s.loomERC721_2.Approve(alice, s.dappchainGateway.Address, aliceTokenID))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC721(alice, aliceTokenID, s.loomERC721_2.Address, nil)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// and receives a withdrawal receipt...
	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	// Alice can now withdraw the token from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetGateway.WithdrawERC721(alice, aliceTokenID, s.mainnetERC721.Address, wr.OracleSignature, validators))

	// Alice should now have her token back on Mainnet
	aliceEndBalance, err := s.mainnetERC721.BalanceOf(alice)
	require.NoError(err)
	require.Equal(big.NewInt(1), aliceEndBalance)

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

// Alice transfers an ERC721X token from Mainnet to DAppChain, then transfers it to Bob on the
// DAppChain, who then withdraws the token from the DAppChain to Mainnet.
func (s *TransferGatewayTestSuite) TestERC721XDepositTransferWithdraw() {
	var err error
	require := s.Require()
	alice := s.alice
	bob := s.bob

	// Give Alice some ERC721X tokens on Mainnet
	tokenID := big.NewInt(100)
	tokenAmt := big.NewInt(5)
	require.NoError(s.mainnetERC721X.MintTokens(s.cardsCreator, tokenID, tokenAmt, alice))
	aliceMainnetERC721XStartBal, err := s.mainnetERC721X.BalanceOf(alice, tokenID)
	require.NoError(err)
	mainnetGatewayStartBal, err := s.mainnetGateway.ERC721XBalance(tokenID, s.mainnetERC721X.Address)
	require.NoError(err)
	aliceLoomERC721XStartBal, err := s.loomERC721X.BalanceOf(alice, tokenID)
	require.NoError(err)

	// Alice deposits some of her tokens to the Mainnet Gateway contract
	require.NoError(s.mainnetERC721X.DepositToGateway(alice, tokenID, tokenAmt))
	depositedAmt, err := s.mainnetGateway.ERC721XBalance(tokenID, s.mainnetERC721X.Address)
	require.NoError(err)
	require.Equal(
		tokenAmt.String(),
		new(big.Int).Sub(depositedAmt, mainnetGatewayStartBal).String(),
		"Alice's tokens should be deposited in the Mainnet Gateway",
	)

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her token in the DAppChain ERC721X contract
	curBalance, err := s.loomERC721X.BalanceOf(alice, tokenID)
	require.NoError(err)
	require.Equal(
		tokenAmt.String(),
		new(big.Int).Sub(curBalance, aliceLoomERC721XStartBal).String(),
		"Alice's tokens should be in the DAppChain ERC721X contract")

	// Alice transfers her tokens to Bob
	require.NoError(s.loomERC721X.TransferFrom(alice, bob, tokenID, tokenAmt))

	// Bob must grant approval to the DAppChain Gateway to take ownership of the tokens when they're withdrawn
	// TODO: Using SetApproveForAll here is shitty, but the sample ERC721X contract doesn't currently
	//       provide the ability to approve the transfer of a specific amount of a certain token ID.
	require.NoError(s.loomERC721X.SetApprovalForAll(bob, s.dappchainGateway.Address, true))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(bob)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Bob can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC721X(bob, tokenID, tokenAmt, s.loomERC721X.Address, &bob.MainnetAddr)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// and receives a withdrawal receipt...
	wr, err := s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.NotNil(wr)

	// Bob revokes prior approval to ensure DAppChain Gateway can't withdraw any more tokens
	require.NoError(s.loomERC721X.SetApprovalForAll(bob, s.dappchainGateway.Address, false))

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	bobMainnetERC721XBal, err := s.mainnetERC721X.BalanceOf(bob, tokenID)
	require.NoError(err)

	// Bob can now withdraw the tokens from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetGateway.WithdrawERC721X(bob, tokenID, tokenAmt, s.mainnetERC721X.Address, wr.OracleSignature, validators))

	// Bob should now have the token Alice sent him, in his Mainnet account
	curBalance, err = s.mainnetGateway.ERC721XBalance(tokenID, s.mainnetERC721X.Address)
	require.NoError(err)
	require.Equal(
		mainnetGatewayStartBal.String(), curBalance.String(),
		"Bob's tokens shouldn't be deposited in the Mainnet Gateway anymore")
	curBalance, err = s.mainnetERC721X.BalanceOf(bob, tokenID)
	require.NoError(err)
	require.Equal(
		curBalance.String(),
		new(big.Int).Add(bobMainnetERC721XBal, tokenAmt).String(),
		"Bob should own Alice's tokens on Mainnet")
	curBalance, err = s.mainnetERC721X.BalanceOf(alice, tokenID)
	require.NoError(err)
	require.Equal(
		curBalance.String(),
		new(big.Int).Sub(aliceMainnetERC721XStartBal, tokenAmt).String(),
		"Alice no longer owns the tokens she sent Bob")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(bob)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Bob's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestLoomDepositAndWithdraw() {
	var err error
	require := s.Require()
	require.NoError(err)

	alice := s.alice

	loomCfg, err := ParseConfig([]string{os.Getenv("LOOM_DIR")})
	require.NoError(err)
	s.dappchainLoomGateway, err = gw.ConnectToDAppChainLoomGateway(s.loomClient, loomCfg.TransferGateway.DAppChainEventsURI)
	require.NoError(err)

	// Give Alice some Loom tokens on Mainnet
	tokenAmount := sciNot(420)
	require.NoError(s.mainnetLoomCoin.Transfer(alice, s.coinCreator, tokenAmount))
	aliceMainnetLoomCoinStartBal, err := s.mainnetLoomCoin.BalanceOf(alice)
	fmt.Println("ALICE MAINNET BALANCE", aliceMainnetLoomCoinStartBal)
	require.NoError(err)
	mainnetLoomGatewayStartBal, err := s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	aliceLoomCoinStartBal, err := s.loomCoin.BalanceOf(alice)
	fmt.Println("ALICE DAPPCHAIN BALANCE", aliceMainnetLoomCoinStartBal)
	require.NoError(err)

	// Alice deposits her tokens into the Mainnet Gateway contract
	require.NoError(s.mainnetLoomCoin.Approve(alice, s.mainnetLoomGateway.Address, tokenAmount))
	require.NoError(s.mainnetLoomGateway.DepositERC20(alice, tokenAmount, s.mainnetLoomCoin.Address))
	curBalance, err := s.mainnetLoomCoin.BalanceOf(alice)
	fmt.Println("ALICE MAINNET BALANCE AFTER DEPOSIT", curBalance)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(aliceMainnetLoomCoinStartBal, curBalance).String(),
		"Alice should no longer have the tokens she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, mainnetLoomGatewayStartBal).String(),
		"Alice's tokens should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her tokens in the DAppChain Loom contract
	curBalance, err = s.loomCoin.BalanceOf(alice)
	fmt.Println("ALICE DAPPCHAIN BALANCE", curBalance)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomCoinStartBal).String(),
		"Alice's tokens should be in the DAppChain Loom contract")

	// TODO: check the token state is correctly set as "deposited"...

	// Alice must grant approval to the DAppChain Gateway to take ownership of the tokens when they're withdrawn
	require.NoError(s.loomCoin.Approve(alice, s.dappchainLoomGateway.Address, tokenAmount))

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainLoomGateway.WithdrawLoom(alice, tokenAmount, s.mainnetLoomCoin.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	wr, err := s.dappchainLoomGateway.WithdrawalReceipt(alice)
	for {
		wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	// Alice can now withdraw the tokens from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetLoomGateway.WithdrawERC20(alice, tokenAmount, s.mainnetLoomCoin.Address, wr.OracleSignature, validators))

	// Alice should now have her tokens back on Mainnet
	curBalance, err = s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	require.Equal(
		mainnetLoomGatewayStartBal.String(), curBalance.String(),
		"Alice's tokens shouldn't be deposited in the Mainnet Gateway anymore")
	aliceEndBalance, err := s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		aliceMainnetLoomCoinStartBal.String(), aliceEndBalance.String(),
		"Alice should have all her tokens in her Mainnet account")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestERC20DepositAndWithdraw() {
	var err error
	require := s.Require()

	alice := s.alice

	// Give Alice some ERC20 tokens on Mainnet
	tokenAmount := sciNot(157)
	require.NoError(s.mainnetCoin.Transfer(alice, s.coinCreator, tokenAmount))
	aliceMainnetCoinStartBal, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	mainnetGatewayStartBal, err := s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	aliceLoomCoinStartBal, err := s.loomERC20.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits her tokens into the Mainnet Gateway contract
	require.NoError(s.mainnetCoin.Approve(alice, s.mainnetGateway.Address, tokenAmount))
	require.NoError(s.mainnetGateway.DepositERC20(alice, tokenAmount, s.mainnetCoin.Address))
	curBalance, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(aliceMainnetCoinStartBal, curBalance).String(),
		"Alice should no longer have the tokens she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, mainnetGatewayStartBal).String(),
		"Alice's tokens should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her tokens in the DAppChain ERC20 contract
	curBalance, err = s.loomERC20.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomCoinStartBal).String(),
		"Alice's tokens should be in the DAppChain ERC20 contract")

	// TODO: check the token state is correctly set as "deposited"...

	// Alice must grant approval to the DAppChain Gateway to take ownership of the tokens when they're withdrawn
	require.NoError(s.loomERC20.Approve(alice, s.dappchainGateway.Address, tokenAmount))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC20(alice, tokenAmount, s.loomERC20.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	// Verify Alice's withdrawal receipt has been signed by the Oracle
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)

	// Alice can now withdraw the tokens from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")
	require.NoError(s.mainnetGateway.WithdrawERC20(alice, tokenAmount, s.mainnetCoin.Address, wr.OracleSignature, validators))

	// Alice should now have her tokens back on Mainnet
	curBalance, err = s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	require.Equal(
		mainnetGatewayStartBal.String(), curBalance.String(),
		"Alice's tokens shouldn't be deposited in the Mainnet Gateway anymore")
	aliceEndBalance, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		aliceMainnetCoinStartBal.String(), aliceEndBalance.String(),
		"Alice should have all her tokens in her Mainnet account")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestERC20MintableWithdraw() {
	var err error
	require := s.Require()

	alice := s.alice

	tokenAmount := sciNot(345)

	aliceMainnetCoinStartBal, err := s.mainnetCoin2.BalanceOf(alice)
	require.NoError(err)

	// Give Alice some ERC20 tokens on Dappchain
	aliceLoomCoinStartBal, err := s.loomERC20_2.BalanceOf(alice)
	require.NoError(s.loomERC20_2.MintTo(s.coinCreator, alice.LoomAddr, tokenAmount))
	require.NoError(err)

	// Alice should now have her tokens in the DAppChain ERC20 contract
	curBalance, err := s.loomERC20_2.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomCoinStartBal).String(),
		"Alice's tokens should be in the DAppChain ERC20 contract")

	// Alice must grant approval to the DAppChain Gateway to take ownership of the tokens when they're withdrawn
	require.NoError(s.loomERC20_2.Approve(alice, s.dappchainGateway.Address, tokenAmount))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC20(alice, tokenAmount, s.loomERC20_2.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	// Verify Alice's withdrawal receipt has been signed by the Oracle
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)

	// Alice can now withdraw the tokens from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")
	require.NoError(s.mainnetGateway.WithdrawERC20(alice, tokenAmount, s.mainnetCoin2.Address, wr.OracleSignature, validators))

	aliceMainnnetEndBalance, err := s.mainnetCoin2.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(), new(big.Int).Sub(aliceMainnnetEndBalance, aliceMainnetCoinStartBal).String(),
		"Alice should have her tokens in her Mainnet account")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestETHDepositAndWithdraw() {
	require := s.Require()
	alice := s.alice

	ethAmount := new(big.Int).Div(sciNot(4), big.NewInt(1000)) // 0.004 ETH

	aliceMainnetEthStartBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	mainnetGatewayStartBal, err := s.mainnetGateway.ETHBalance()
	require.NoError(err)
	aliceLoomEthStartBal, err := s.loomEth.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits some ETH into the Mainnet Gateway contract
	txFee, err := s.mainnetGateway.DepositETH(alice, ethAmount)
	require.NoError(err)
	curBalance, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Add(ethAmount, txFee).String(),
		new(big.Int).Sub(aliceMainnetEthStartBal, curBalance).String(),
		"Alice should no longer have the ETH she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		ethAmount.String(),
		new(big.Int).Sub(curBalance, mainnetGatewayStartBal).String(),
		"Alice's ETH should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her ETH in the DAppChain ETH contract
	curBalance, err = s.loomEth.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		ethAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomEthStartBal).String(),
		"Alice's ETH should be in the DAppChain ETH contract")

	// Alice must grant approval to the DAppChain Gateway to take ownership of the ETH she wishes to
	// withdraw to Mainnet
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, ethAmount))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawETH(alice, ethAmount, s.mainnetGateway.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetEthBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)

	// Alice can now withdraw the ETH from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	txFee, err = s.mainnetGateway.WithdrawETH(alice, ethAmount, wr.OracleSignature, validators)
	require.NoError(err)
	// Alice should now have her ETH back on Mainnet
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		mainnetGatewayStartBal.String(), curBalance.String(),
		"Alice's ETH shouldn't be deposited in the Mainnet Gateway anymore")
	curBalance, err = s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetEthBal).String(),
		new(big.Int).Sub(ethAmount, txFee).String(),
		"Alice should have all her ETH in her Mainnet account (minus tx fees)")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

// Similar to TestETHDepositAndWithdraw, but with a few extra balance/transfer EVM ops thrown in to
// make sure ethcoin/EVM integration is working as expected.
func (s *TransferGatewayTestSuite) TestETHDepositAndWithdrawWithEVM() {
	require := s.Require()

	evmTestContract, err := evm_eth.DeployEthEvmTestContractToDAppChain(s.loomClient, s.cardsCreator.LoomSigner)
	require.NoError(err)

	alice := s.alice
	bob := s.bob

	aliceMainnetEthStartBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	mainnetGatewayStartBal, err := s.mainnetGateway.ETHBalance()
	require.NoError(err)
	aliceLoomEthStartBal, err := s.loomEth.BalanceOf(alice)
	require.NoError(err)

	ethAmount := new(big.Int).Div(sciNot(45), big.NewInt(10000)) // 0.0045 ETH

	// Alice deposits some ETH into the Mainnet Gateway contract
	txFee, err := s.mainnetGateway.DepositETH(alice, ethAmount)
	require.NoError(err)
	curBalance, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Add(ethAmount, txFee).String(),
		new(big.Int).Sub(aliceMainnetEthStartBal, curBalance).String(),
		"Alice should no longer have the ETH she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		ethAmount.String(),
		new(big.Int).Sub(curBalance, mainnetGatewayStartBal).String(),
		"Alice's ETH should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her ETH in the DAppChain ETH contract
	curBalance, err = evmTestContract.Balance(alice.LoomAddr)
	require.NoError(err)
	require.Equal(
		ethAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomEthStartBal).String(),
		"Alice's ETH should be available in the Loom EVM")
	alicePrevBal := curBalance

	// Alice transfers the ETH to Bob (in a roundabout way via EVM test contract)
	// TODO: replace loomEth.Transfer with evmTestContract.Deposit when that's implemented
	contractStartBal, err := evmTestContract.Balance(evmTestContract.Address)
	require.NoError(s.loomEth.Transfer(alice, evmTestContract.Address, ethAmount))

	curBalance, err = evmTestContract.Balance(alice.LoomAddr)
	require.NoError(err)
	require.Equal(
		amountAsString(ethAmount),
		amountAsString(new(big.Int).Sub(alicePrevBal, curBalance)),
		"Alice should no longer have ETH she transferred to the EVM contract")
	alicePrevBal = curBalance

	curBalance, err = evmTestContract.Balance(evmTestContract.Address)
	require.NoError(err)
	require.Equal(
		amountAsString(ethAmount),
		amountAsString(new(big.Int).Sub(curBalance, contractStartBal)),
		"EVM contract should've received Alice's ETH")

	contractPrevBal := curBalance
	bobPrevBal, err := evmTestContract.Balance(bob.LoomAddr)
	require.NoError(err)
	require.NoError(evmTestContract.Withdraw(bob, ethAmount))

	curBalance, err = evmTestContract.Balance(evmTestContract.Address)
	require.NoError(err)
	require.Equal(
		amountAsString(ethAmount),
		amountAsString(new(big.Int).Sub(contractPrevBal, curBalance)),
		"EVM contract should no longer have ETH withdrawn by Bob")

	curBalance, err = evmTestContract.Balance(bob.LoomAddr)
	require.NoError(err)
	require.Equal(
		amountAsString(ethAmount),
		amountAsString(new(big.Int).Sub(curBalance, bobPrevBal)),
		"Bob should've received ETH from the EVM contract")

	// Bob sends the ETH back to Alice...
	// TODO: replace loomEth.Transfer with evmTestContract.Deposit when that's implemented
	require.NoError(s.loomEth.Transfer(bob, evmTestContract.Address, ethAmount))
	contractPrevBal, err = evmTestContract.Balance(evmTestContract.Address)
	require.NoError(err)
	// and something blows up while transferring it to Alice...
	require.Error(evmTestContract.WithdrawThenFail(alice, ethAmount))
	// the contract should still have all the ETH Bob sent, and Alice shouldn't have received any
	curBalance, err = evmTestContract.Balance(evmTestContract.Address)
	require.NoError(err)
	require.Equal(amountAsString(contractPrevBal), amountAsString(curBalance))
	curBalance, err = evmTestContract.Balance(alice.LoomAddr)
	require.NoError(err)
	require.Equal(amountAsString(alicePrevBal), amountAsString(curBalance))

	// Give Alice her ETH for real this time
	require.NoError(evmTestContract.Withdraw(alice, ethAmount))

	// Alice must grant approval to the DAppChain Gateway to take ownership of the ETH she wishes to
	// withdraw to Mainnet
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, ethAmount))

	// Wait until the receipt is empty
	for {
		wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
		require.NoError(err)
		if wr == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawETH(alice, ethAmount, s.mainnetGateway.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// and receives a withdrawal receipt...
	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}

	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.NotNil(wr)

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetEthBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)

	// Alice can now withdraw the ETH from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	txFee, err = s.mainnetGateway.WithdrawETH(alice, ethAmount, wr.OracleSignature, validators)
	require.NoError(err)
	// Alice should now have her ETH back on Mainnet
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		mainnetGatewayStartBal.String(), curBalance.String(),
		"Alice's ETH shouldn't be deposited in the Mainnet Gateway anymore")
	curBalance, err = s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetEthBal).String(),
		new(big.Int).Sub(ethAmount, txFee).String(),
		"Alice should have all her ETH in her Mainnet account (minus tx fees)")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

func (s *TransferGatewayTestSuite) TestETHWithdrawalLimit() {
	require := s.Require()
	alice := s.alice

	// current max total withdrawal amount is 1,000,000
	// current max per account withdrawal amount is 500,000
	amount := sciNot(2000000)

	aliceMainnetEthStartBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	gatewayMainnetEthStartBal, err := s.mainnetGateway.ETHBalance()
	require.NoError(err)
	aliceDappchainEthStartBal, err := s.loomEth.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits some ETH into the Mainnet Gateway contract
	txFee, err := s.mainnetGateway.DepositETH(alice, amount)
	require.NoError(err)
	curBalance, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Add(amount, txFee).String(),
		new(big.Int).Sub(aliceMainnetEthStartBal, curBalance).String(),
		"Alice should no longer have the ETH she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		amount.String(),
		new(big.Int).Sub(curBalance, gatewayMainnetEthStartBal).String(),
		"Alice's ETH should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her ETH in the DAppChain ETH contract
	curBalance, err = s.loomEth.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		amount.String(),
		new(big.Int).Sub(curBalance, aliceDappchainEthStartBal).String(),
		"Alice's ETH should be in the DAppChain ETH contract")

	// Alice must grant approval to the DAppChain Gateway to take ownership of the ETH she wishes to
	// withdraw to Mainnet
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, amount))

	amount2 := sciNot(200000)
	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawETH(alice, amount2, s.mainnetGateway.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	var wr *tgtypes.TransferGatewayWithdrawalReceipt
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
	require.NoError(err)
	require.NotNil(wr)

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetEthBal, err := s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)

	// Alice can now withdraw the ETH from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	txFee, err = s.mainnetGateway.WithdrawETH(alice, amount2, wr.OracleSignature, validators)
	require.NoError(err)
	// Alice should now have her ETH back on Mainnet
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(amount, amount2).String(), curBalance.String(),
		"Alice's ETH should be withdrawn to Mainnet Gateway and still have some amount on Dappchain Gateway")
	curBalance, err = s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetEthBal).String(),
		new(big.Int).Sub(amount2, txFee).String(),
		"Alice should have all her ETH in her Mainnet account (minus tx fees)")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")

	// case1: Withdrawal amount exceeds max daily total limit
	amount3 := sciNot(1300000)
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, amount3))
	err = s.dappchainGateway.WithdrawETH(alice, amount3, s.mainnetGateway.Address)
	require.Error(err, "withdraw ETH should fail")
	require.True(strings.Contains(err.Error(), "TG024"), "Alice should not be able to withdraw ETH because the withdrawal amount exceeds that of daily total limit")

	// case2: Withdrawal amount exceeds max daily per account limit
	amount3 = sciNot(400000) // 400K ETH
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, amount3))
	err = s.dappchainGateway.WithdrawETH(alice, amount3, s.mainnetGateway.Address)
	require.Error(err, "withdraw ETH should fail")
	require.True(strings.Contains(err.Error(), "TG025"), "Alice should not be able to withdraw ETH because the withdrawal amount exceeds that of daily per account limit")

	// case3: Withdrawal should just work if the limit is not reached
	amount3 = sciNot(200000) // total sum should be 200,000 + 200,000 which is not reached the limit 500,000
	require.NoError(s.loomEth.Approve(alice, s.dappchainGateway.Address, amount3))

	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawETH(alice, amount3, s.mainnetGateway.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetEthBal, err = s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)

	// Alice can now withdraw the ETH from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	txFee, err = s.mainnetGateway.WithdrawETH(alice, amount3, wr.OracleSignature, validators)
	require.NoError(err)
	// Alice should now have her ETH back on Mainnet
	curBalance, err = s.mainnetGateway.ETHBalance()
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(amount, new(big.Int).Add(amount3, amount2)).String(),
		curBalance.String(),
		"Alice's ETH should be withdrawn to Mainnet Gateway and still have some amount on Dappchain Gateway")
	curBalance, err = s.ethClient.BalanceAt(context.TODO(), alice.MainnetAddr, nil)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetEthBal).String(),
		new(big.Int).Sub(amount3, txFee).String(),
		"Alice should have all her ETH in her Mainnet account (minus tx fees)")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)
}

func (s *TransferGatewayTestSuite) TestLoomCoinWithdrawalLimit() {
	require := s.Require()
	alice := s.alice

	loomCfg, err := ParseConfig([]string{os.Getenv("LOOM_DIR")})
	require.NoError(err)
	s.dappchainLoomGateway, err = gw.ConnectToDAppChainLoomGateway(s.loomClient, loomCfg.TransferGateway.DAppChainEventsURI)
	require.NoError(err)

	// current max total withdrawal amount is 1,000,000
	// current max per account withdrawal amount is 500,000
	amount := sciNot(2000000)

	require.NoError(s.mainnetLoomCoin.Transfer(alice, s.coinCreator, amount))
	aliceMainnetEthStartBal, err := s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)
	gatewayMainnetLoomCoinStartBal, err := s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	aliceDappchainLoomCoinStartBal, err := s.loomCoin.BalanceOf(alice)
	require.NoError(err)

	// Alice deposits some LOOM into the Mainnet Gateway contract
	require.NoError(s.mainnetLoomCoin.Approve(alice, s.mainnetLoomGateway.Address, amount))
	require.NoError(s.mainnetLoomGateway.DepositERC20(alice, amount, s.mainnetLoomCoin.Address))

	curBalance, err := s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		amount.String(),
		new(big.Int).Sub(aliceMainnetEthStartBal, curBalance).String(),
		"Alice should no longer have the LOOM she deposited to the Mainnet Gateway")
	curBalance, err = s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	require.Equal(
		amount.String(),
		new(big.Int).Sub(curBalance, gatewayMainnetLoomCoinStartBal).String(),
		"Alice's LOOM should be deposited in the Mainnet Gateway")

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her LOOM in the DAppChain LOOMCOIN contract
	curBalance, err = s.loomCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		amount.String(),
		new(big.Int).Sub(curBalance, aliceDappchainLoomCoinStartBal).String(),
		"Alice's LOOM should be in the DAppChain LOOMCOIN contract")

	// Alice must grant approval to the DAppChain Gateway to take ownership of the LOOM she wishes to
	// withdraw to Mainnet
	require.NoError(s.loomCoin.Approve(alice, s.dappchainLoomGateway.Address, amount))

	amount2 := sciNot(200000)
	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainLoomGateway.WithdrawLoom(alice, amount2, s.mainnetLoomCoin.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	var wr *tgtypes.TransferGatewayWithdrawalReceipt
	for {
		wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
	require.NoError(err)
	require.NotNil(wr)

	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetLoomCoinBal, err := s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)

	// Alice can now withdraw the LOOM from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetLoomGateway.WithdrawERC20(alice, amount2, s.mainnetLoomCoin.Address, wr.OracleSignature, validators))

	// Alice should now have her LOOM back on Mainnet
	curBalance, err = s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(amount, amount2).String(), curBalance.String(),
		"Alice's LOOM should be withdrawn to Mainnet Gateway and still have some amount on Dappchain Gateway")
	curBalance, err = s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetLoomCoinBal).String(),
		amount2.String(),
		"Alice should have all her LOOM in her Mainnet account (minus tx fees)")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")

	// case1: Withdrawal amount exceeds max daily total limit
	amount3 := sciNot(1300000)
	require.NoError(s.loomCoin.Approve(alice, s.dappchainLoomGateway.Address, amount3))
	err = s.dappchainLoomGateway.WithdrawLoom(alice, amount3, s.mainnetLoomCoin.Address)
	require.Error(err, "withdraw LOOM should fail")
	require.True(strings.Contains(err.Error(), "TG024"), "Alice should not be able to withdraw LOOM because the withdrawal amount exceeds that of daily total limit")

	// case2: Withdrawal amount exceeds max daily per account limit
	amount3 = sciNot(400000) // 400K LOOM
	require.NoError(s.loomCoin.Approve(alice, s.dappchainLoomGateway.Address, amount3))
	err = s.dappchainLoomGateway.WithdrawLoom(alice, amount3, s.mainnetLoomCoin.Address)
	require.Error(err, "withdraw LOOM should fail")
	require.True(strings.Contains(err.Error(), "TG025"), "Alice should not be able to withdraw LOOM because the withdrawal amount exceeds that of daily per account limit")

	// case3: Withdrawal should just work if the limit is not reached
	amount3 = sciNot(200000) // total sum should be 200,000 + 200,000 which is not reached the limit 500,000
	require.NoError(s.loomCoin.Approve(alice, s.dappchainLoomGateway.Address, amount3))

	// Now Alice can request a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainLoomGateway.WithdrawLoom(alice, amount3, s.mainnetLoomCoin.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	// Let the Oracle fetch pending withdrawals & sign them
	for {
		wr, err = s.dappchainLoomGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
	require.NoError(err)
	require.NotNil(wr)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")

	aliceMainnetLoomCoinBal, err = s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)

	// Alice can now withdraw the LOOM from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	require.NoError(s.mainnetLoomGateway.WithdrawERC20(alice, amount3, s.mainnetLoomCoin.Address, wr.OracleSignature, validators))
	// Alice should now have her LOOM back on Mainnet
	curBalance, err = s.mainnetLoomGateway.ERC20Balance(s.mainnetLoomCoin.Address)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(amount, new(big.Int).Add(amount3, amount2)).String(),
		curBalance.String(),
		"Alice's LOOM should be withdrawn to Mainnet Gateway and still have some amount on Dappchain Gateway")
	curBalance, err = s.mainnetLoomCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		new(big.Int).Sub(curBalance, aliceMainnetLoomCoinBal).String(),
		amount3.String(),
		"Alice should have all her LOOM in her Mainnet account")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)
}

func (s *TransferGatewayTestSuite) TestHotWalletERC20DepositWithdraw() {
	var err error
	require := s.Require()

	alice := s.alice

	// Give Alice some ERC20 tokens on Mainnet
	tokenAmount := sciNot(300)
	tokenAmountHalf := sciNot(150)
	require.NoError(s.mainnetCoin.Transfer(alice, s.coinCreator, tokenAmount))
	aliceMainnetCoinStartBal, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	mainnetGatewayStartBal, err := s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	aliceLoomCoinStartBal, err := s.loomERC20.BalanceOf(alice)
	require.NoError(err)

	// Alice sends her tokens into the Mainnet Gateway contract
	// Split into 2 txs to test if Gateway supports multiple deposits per user
	tx1, err := s.mainnetCoin.TransferTx(alice, s.mainnetGateway.Address, tokenAmountHalf)
	require.NoError(err)
	tx2, err := s.mainnetCoin.TransferTx(alice, s.mainnetGateway.Address, tokenAmountHalf)
	require.NoError(err)

	curBalance, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(aliceMainnetCoinStartBal, curBalance).String(),
		"Alice should no longer have the tokens she deposited to the Mainnet Gateway")

	curBalance, err = s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, mainnetGatewayStartBal).String(),
		"Alice's tokens should be deposited in the Mainnet Gateway")

	// Alice submits tx hash to Gateway
	require.NoError(s.dappchainGateway.SubmitHotWalletDepositTxHash(alice, tx1.Hash()),
		"Alice should be able to submit hot wallet tx1 hash to Gateway",
	)
	require.NoError(s.dappchainGateway.SubmitHotWalletDepositTxHash(alice, tx2.Hash()),
		"Alice should be able to submit hot wallet tx2 hash to Gateway",
	)

	// Alice submits pending tx hash to Gateway, which should not work
	require.Error(s.dappchainGateway.SubmitHotWalletDepositTxHash(alice, tx1.Hash()),
		"Alice should NOT be able to submit pending hot wallet tx1 hash to Gateway",
	)
	require.Error(s.dappchainGateway.SubmitHotWalletDepositTxHash(alice, tx2.Hash()),
		"Alice should NOT be able to submit pending hot wallet tx2 hash to Gateway",
	)

	// Let the Oracle notify the DAppChain Gateway about Alice's deposit
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Alice should now have her tokens in the DAppChain ERC20 contract
	curBalance, err = s.loomERC20.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		tokenAmount.String(),
		new(big.Int).Sub(curBalance, aliceLoomCoinStartBal).String(),
		"Alice's tokens should be in the DAppChain ERC20 contract")

	// Alice submits already processed tx hash to Gateway, which should not work
	// This test requires tg:check-txhash to be enabled.
	require.Error(s.dappchainGateway.SubmitHotWalletDepositTxHash(alice, tx1.Hash()),
		"Alice should NOT be able to submit processed hot wallet tx1 hash to Gateway",
	)

	// Alice must grant approval to the DAppChain Gateway to take ownership of the tokens when they're withdrawn
	require.NoError(s.loomERC20.Approve(alice, s.dappchainGateway.Address, tokenAmount))

	wr, err := s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")

	// Now Alice can requests a withdrawal from the DAppChain Gateway...
	for i := 0; i < 5; i++ {
		err = s.dappchainGateway.WithdrawERC20(alice, tokenAmount, s.loomERC20.Address)
		if err != nil {
			if strings.Contains(err.Error(), "TG003") {
				time.Sleep(5 * time.Second)
			} else {
				require.NoError(err)
			}
		} else {
			break
		}
	}
	require.NoError(err)

	for {
		wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
		if wr.OracleSignature != nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
	require.NoError(err)

	// Alice can now withdraw the tokens from the Mainnet Gateway by presenting the signature from
	// the withdrawal receipt
	validators, err := s.validatorsManager.GetValidators()
	require.NoError(err)

	// Verify Alice's withdrawal receipt has been signed by enough validators
	require.True(len(wr.OracleSignature)/65 >= 2*len(validators)/3, "Must be signed by 2/3rds validators")
	require.NoError(s.mainnetGateway.WithdrawERC20(alice, tokenAmount, s.mainnetCoin.Address, wr.OracleSignature, validators))

	// Alice should now have her tokens back on Mainnet
	curBalance, err = s.mainnetGateway.ERC20Balance(s.mainnetCoin.Address)
	require.NoError(err)
	require.Equal(
		mainnetGatewayStartBal.String(), curBalance.String(),
		"Alice's tokens shouldn't be deposited in the Mainnet Gateway anymore")
	aliceEndBalance, err := s.mainnetCoin.BalanceOf(alice)
	require.NoError(err)
	require.Equal(
		aliceMainnetCoinStartBal.String(), aliceEndBalance.String(),
		"Alice should have all her tokens in her Mainnet account")

	// Let the Oracle notify the DAppChain Gateway that Alice has completed the withdrawal
	s.mineBlocksTillConfirmation()
	time.Sleep(s.oracleWaitTime)

	// Check the DAppChain Gateway has been updated...
	wr, err = s.dappchainGateway.WithdrawalReceipt(alice)
	require.NoError(err)
	require.Nil(wr, "DAppChain Gateway should've cleared out Alice's pending withdrawal")
}

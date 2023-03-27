// The SDS Spaghetti module fetches the blockchain data and converts it into the internal format
// All other SDS Services are connecting to SDS Spaghetti.
//
// We have multiple workers.
// Atleast one worker for each network.
// This workers are called recent workers.
//
// Categorizer checks whether the cached block returned or not.
// If its a cached block, then switches to the block_range
package blockchain

import (
	"github.com/blocklords/sds/app/log"
	common_command "github.com/blocklords/sds/app/remote/command"
	"github.com/blocklords/sds/blockchain/command"
	blockchain_process "github.com/blocklords/sds/blockchain/inproc"
	"github.com/blocklords/sds/categorizer"

	"github.com/blocklords/sds/blockchain/network"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/app/service"

	"github.com/blocklords/sds/app/controller"
	"github.com/blocklords/sds/app/remote"
	"github.com/blocklords/sds/app/remote/message"

	"fmt"

	evm_categorizer "github.com/blocklords/sds/blockchain/evm/categorizer"
	imx_categorizer "github.com/blocklords/sds/blockchain/imx/categorizer"

	evm_client "github.com/blocklords/sds/blockchain/evm/client"
	imx_client "github.com/blocklords/sds/blockchain/imx/client"

	"github.com/blocklords/sds/blockchain/imx"
	imx_worker "github.com/blocklords/sds/blockchain/imx/worker"
)

////////////////////////////////////////////////////////////////////
//
// Command handlers
//
////////////////////////////////////////////////////////////////////

// this function returns the smartcontract deployer, deployed block number
// and block timestamp by a transaction hash of the smartcontract deployment.
func transaction_deployed_get(request message.Request, logger log.Logger, parameters ...interface{}) message.Reply {
	var request_parameters command.DeployedTransaction
	err := request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters " + err.Error())
	}

	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return message.Fail("network: " + err.Error())
	}

	if !networks.Exist(request_parameters.NetworkId) {
		return message.Fail("unsupported network id")
	}

	app_config := parameters[0].(*configuration.Config)
	url := blockchain_process.BlockchainManagerUrl(request_parameters.NetworkId)
	sock, err := remote.InprocRequestSocket(url, logger, app_config)
	if err != nil {
		return message.Fail("blockchain request error: " + err.Error())
	}
	defer sock.Close()

	req_parameters := command.Transaction{
		TransactionId: request_parameters.TransactionId,
	}

	var blockchain_reply command.LogFilterReply
	err = command.FILTER_LOG_COMMAND.Request(sock, req_parameters, &blockchain_reply)
	if err != nil {
		return message.Fail("remote transaction_request: " + err.Error())
	}

	reply, err := common_command.Reply(blockchain_reply)
	if err != nil {
		return message.Fail("reply preparation: " + err.Error())
	}

	return reply
}

// Returns Network
func get_network(request message.Request, logger log.Logger, _ ...interface{}) message.Reply {
	command_logger, err := logger.ChildWithoutReport("network-get-command")
	if err != nil {
		return message.Fail("network-get-command: " + err.Error())
	}
	command_logger.Info("incoming request", "parameters", request.Parameters)

	var request_parameters command.NetworkId
	err = request.Parameters.ToInterface(&request_parameters)
	if err != nil {
		return message.Fail("failed to parse request parameters " + err.Error())
	}

	networks, err := network.GetNetworks(request_parameters.NetworkType)
	if err != nil {
		return message.Fail(err.Error())
	}

	n, err := networks.Get(request_parameters.NetworkId)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := command.NetworkReply{
		Network: *n,
	}
	reply_message, err := common_command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_network_ids(request message.Request, _ log.Logger, _ ...interface{}) message.Reply {
	var parameters command.NetworkIds
	err := request.Parameters.ToInterface(&parameters)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	network_ids, err := network.GetNetworkIds(parameters.NetworkType)
	if err != nil {
		return message.Fail(err.Error())
	}

	reply := command.NetworkIdsReply{
		NetworkIds: network_ids,
	}
	reply_message, err := common_command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Returns an abi by the smartcontract key.
func get_all_networks(request message.Request, logger log.Logger, _ ...interface{}) message.Reply {
	command_logger, err := logger.ChildWithoutReport("network-get-all-command")
	if err != nil {
		return message.Fail("network-get-all-command: " + err.Error())
	}
	command_logger.Info("incoming request", "parameters", request.Parameters)

	var parameters command.NetworkIds
	err = request.Parameters.ToInterface(&parameters)
	if err != nil {
		return message.Fail("invalid parameters: " + err.Error())
	}

	networks, err := network.GetNetworks(parameters.NetworkType)
	if err != nil {
		return message.Fail("blockchain " + err.Error())
	}

	reply := command.NetworksReply{
		Networks: networks,
	}
	reply_message, err := common_command.Reply(reply)
	if err != nil {
		return message.Fail("failed to reply: " + err.Error())
	}

	return reply_message
}

// Return the list of command handlers for this service
func CommandHandlers() controller.CommandHandlers {
	var commands = controller.CommandHandlers{
		"transaction_deployed_get": transaction_deployed_get,
		"network_id_get_all":       get_network_ids,
		"network_get_all":          get_all_networks,
		"network_get":              get_network,
	}

	return commands
}

// Returns this service's configuration
func Service() *service.Service {
	return service.Inprocess(service.SPAGHETTI)
}

func Run(app_config *configuration.Config) {
	logger, _ := log.New("blockchain", log.WITH_TIMESTAMP)

	logger.Info("starting")

	this_service := Service()
	reply, err := controller.NewReply(this_service, logger)
	if err != nil {
		logger.Fatal("controller new", "message", err)
	}

	err = run_networks(logger, app_config)
	if err != nil {
		logger.Fatal("StartWorkers", "message", err)
	}

	err = reply.Run(CommandHandlers(), app_config)
	if err != nil {
		logger.Fatal("controller error", "message", err)
	}
}

// Start the workers for each blockchain
func run_networks(logger log.Logger, app_config *configuration.Config) error {
	networks, err := network.GetNetworks(network.ALL)
	if err != nil {
		return fmt.Errorf("gosds/blockchain: failed to get networks: %v", err)
	}

	imx_network_found := false

	// if there are some logs, we should broadcast them to the SDS Categorizer
	pusher, err := categorizer.NewCategorizerPusher()
	if err != nil {
		logger.Fatal("create a pusher to SDS Categorizer", "message", err)
	}

	for _, new_network := range networks {
		worker_logger, err := logger.ChildWithTimestamp(new_network.Type.String() + "_network_id_" + new_network.Id)
		if err != nil {
			return fmt.Errorf("child logger: %w", err)
		}

		if new_network.Type == network.EVM {
			blockchain_manager, err := evm_client.NewManager(new_network, worker_logger, app_config)
			if err != nil {
				return fmt.Errorf("gosds/blockchain: failed to create EVM client: %v", err)
			}
			go blockchain_manager.SetupSocket()

			// Categorizer of the smartcontracts
			// This categorizers are interacting with the SDS Categorizer
			categorizer, err := evm_categorizer.NewManager(worker_logger, new_network, pusher, app_config)
			if err != nil {
				worker_logger.Fatal("evm categorizer manager", "error", err)
			}
			go categorizer.Start()
		} else if new_network.Type == network.IMX {
			imx_network_found = true

			new_client := imx_client.New(new_network)

			new_worker := imx_worker.New(app_config, new_client, worker_logger)
			go new_worker.SetupSocket()

			imx_manager, err := imx_categorizer.NewManager(worker_logger, app_config, new_network, pusher)
			if err != nil {
				worker_logger.Fatal("imx.NewManager", "error", err)
			}
			go imx_manager.Start()
		} else {
			return fmt.Errorf("no blockchain handler for network_type %v", new_network.Type)
		}
	}

	if imx_network_found {
		err = imx.ValidateEnv(app_config)
		if err != nil {
			return fmt.Errorf("gosds/blockchain: failed to validate IMX specific config: %v", err)
		}
	}

	return nil
}

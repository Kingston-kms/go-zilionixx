package launcher

import "gopkg.in/urfave/cli.v1"

// GenTestNetGenesisBlock generate the testnet genesis file
var GenTestNetGenesisBlock = cli.BoolTFlag{
	Name:  "gentest",
	Usage: "Generate the genesis file of testnet.",
}

// GenMainNetGenesisBlock generate the testnet genesis file
var GenMainNetGenesisBlock = cli.BoolTFlag{
	Name:  "genmain",
	Usage: "Generate the genesis file of mainnet.",
}

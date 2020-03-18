/**
 * Usage Examples
 * 
 * truffle exec scripts/check_validator.js <gateway-eth-addr> <validator-addr>
 * 
 * To if an address belongs to a TG validator:
 * ./node_modules/.bin/truffle exec scripts/check_validator.js \
 *   0xf5cAD0DB6415a71a5BC67403c87B56b629b4DdaA 0x7292694902bcaf4e1620629e7198cdcb3f572a24 \
 *   --network rpc
 */
const Gateway = artifacts.require('Gateway')

// fetched from etherscan: http://api.etherscan.io/api?module=account&action=txlist&address=0x223ca78df868367d214b444d561b9123c018963a&startblock=0&endblock=99999999&sort=asc&apikey=4H9D2FKQFXK2PJZ63MDYQ1IRB8EUCYSBUR
const candidates = ["0x620118ad98f2f2c02c823edf96555f8c6494c163","0xf1459f9efdc06c7c278a1e9d6187050efe517ab5","0x6db6cc4e2a184300573fc7d378eb13f68fcd5a10","0x9334450968ac2e3b6eb0cb70ad813f27d8576915","0x925437ead20d406ab091e8308324b4bd37e4f477","0x2e77cf8e5536732627544d13edceb16dec748763","0x12fd4276eb3cbbf4f2370abe2238bcdbbf347375","0xa3a3c6df9fe3e22e9ef4df63bb1151ef47e2cf54","0x8e5d30f161ba3ebb09dc3c1f06515656af34baa1","0x9aca21d4c170c80610593640ca3f1afd23a19562","0xf4ae6165c3e05ae7426c5b5200e8b9a7565ac595","0x68d36dcbdd7bbf206e27134f28103abe7cf972df","0x215c22ab36103f1faabcd0b8dd4e0e7e642678ef","0x51e723b14b160b9b420cbf7ccdb889f36213f0f3","0x53422d7da1334bab84a5ee9b70e1c5f39f635a0b","0x64208af1e2ea99c4ca7f5a6572b391b913de7ae3","0xcf0e9b4746cfb97bae329fe5f696969f6564566a","0x2b4a3ebb0b5be22e53d058029948d882b4afe27b","0x12b65a6296f03a5a984a818664ec98ab20ba4de6","0x1814e872e8c5775a32b15765800e054b83272d7f","0x557238f2c76a73294ddb58b9b8a248beafd69cb9","0x35f6dbc267e106e73c46838e0ba4ef460db3fe04","0x948de4974915313439e108ddbea6671aa36e53b7","0xcf1f33b751789b1d847222dd528535800b76f998","0x5cdb0039db97409243ca00ca70be8616fba322a0","0x908fd4558d7ef6b31d68ba50c70a85723ef4aad9","0x69930e568d593f3ad6c261e7e213dda8b47e9435","0x9d24f0c47c47ad368f8dd0b0af6000732aaecccb","0xd63b87fa46652a28cb3ffcbc56f7721fc60943ad","0xff60e3b341474478628680756e8fcf16e7d64a0e","0x42e62e421bedf2469826879ec1a0574d7d3cca26","0x5fd1fbd87a17657e8f36acbc0e5c09a83676f680","0xa5626ef329769866f95e7e0dc05ee11e102dc700","0x08492321b38414a116d093677eb740054e2d0929","0x51f80c672f2f5105a8d4c09c484f9891749a0b2e","0xb1c45f12548efe0213165ca2d0198a3c8ec499f0","0x35831dd1b909058c06d1a81d652bae40c10f70df","0x0995ce63f1f39b43b25e4cf19a454710d66482b5","0xd22886236f453e9407f54cc2706b2e9c87789702","0x25da25e0324512eb3051ffbba26fc2851e590e06","0x9970c3f6f51c7aec94c5200be00f40fc64496703","0x1187a364dcb403897574ac135ea39549dacd8f0f","0xa669a93d0604ea4efe8a03053647a13a0c6e0870","0x0ea78e7a0a9cb270aa015df2cf5cf6b5c86b7091","0xe310ecb5ad4bdf59d7e2bfa28871a67d260ecef0","0xb07e9392ff0b3ebb551716922042a8578fd21ba2","0x3a22282cbd2715e9d302b4a4ab0d6a117d8438b6","0xd8d908959b0fdaeb9e5812443841362c6ef6663d","0x5afd26b68833d22e9b5ef0e6d2a661b81214e5c5","0x2e9aa156809a7f65f4b420aacc0188b0c8605326","0x7beed30332656ca4220cb2ce8e4508fb18013e8d","0x52af787439a82f36d6ef6b0da0f0e5ecce29ff90","0x563153823d702516f92fc24edd9358d6973f60f9","0x5c23ded0121e059269e3ebcb479239cd1499011c","0x575c3342462010ca04e6ff0885f12cd88e92c585","0xfc33d33a251bf20334381292daed3165a460e673","0x4ebdf0df8e94e3cb45502edf90de083583995879","0x95b810ae711ac53b10f147a9e9558bb67568672e","0x242ba9e89adfabbf94769aa277199a352e9296af","0x123085670e817602041e58febb243abe01a9a825","0x758fdfb34badc3310066fcff4cd8daa1af22e66e","0x7262d4c97c7b93937e4810d289b7320e9da82857","0x5fdd054c475360fe2aea5cac7ced8c24c5cba706","0xd6877ffd13b4b083dbe27e9f98382ce95af30db2","0x0be2bc95ea604a5ac4ecce0f8570fe58bc9c320a","0x7aa6fc0ecec21138a68d4beb6541156be38aa36c","0x66f5389c4ce7c9c0fc322eaa0106a8d665775489","0x8b1510d9aaf015f23acf13e328ffb5ab065c5bd9","0xa7b33cd26f27f1c6b709db5cae442e42387ba69a","0xdcc25317980744fb683c5027a1f740e1904d0422","0x61815db3ceada8fe04c9fac89a65bc7fedc92ff5","0x4edbf9fee4dd4f8cf8d93bce66551bc87a254005","0xa475b33b07629ad300b1d70d55778237d8a6ab14","0x2614abecffaa3a938b8fbe0b1eaaa7a61e42f7a7","0x3bf03010cb0ff616e2a9806efee4920ff67cb651","0x7d64d706341d7ff40e840f6cff535d3d1a4f71c6","0xc7391970d642faf65fabac8f63b0d41c4481d787","0x6f6ecc07486df848668bcf3380ca7d9ae5de01f3","0xee5348348dcfc215eb7e3ff011bb4e44e9f77878","0x08ab98f40b377cb09ced9c147911bdb2ccd31fec","0x3aaed2fa94a012dc322e297924aea8a6d019ba59","0x9e404e86127931ac193da80c26e49922090d97a0","0x296e98424845b0d6fa6179aa92ecb83a5667434d","0xa548a329601a6d348b9bfcbf4229a2c6fbdec022","0xca4a49ddab98208101bc3cea75075f397c2aeef8","0x2067bed542762d26e2755ce7d8776728f3429f48","0x7f297560fc715ec0cebc684e486db1ceb64c2a0b","0x25af3bca65962b4dbeb105175198090fefca7e47","0x06447c96d050581f922e16ec12decf47bdf2cf05","0x44f248d602a7e7bb968426a6cf3d4e962ae0036b","0x8a6e9b730eea6753af9a4a2d29a20bc8760c412e","0x082c61297150d5fe458f64b08ed075573e03f609","0x4d25819e9de2a2eeeddca953d1ef0182680a7054","0xa256fe215dc740956cb3c6087a6d3bf115786616","0x909850e31836fd81c404d5ed72d263c74e43183c","0x218a4a788f7ccefb686324aa90e0a27984c88e9e","0x35e352e52fc7b7daa82c8e875c45294800bc530b","0xbad858a0cf09f210fcf35cbf83569178879b47f2","0x770cc1fbc6c576fd4b2c15b30f09a54916cf9659","0x4906e4f95ad546ce865916f65c825e00630bffa8","0x49caec9a29ea14c2420892e15597b0fcb3cf7c59","0xd34a12eced1ee22a9007689a8a6d5fe902f58307","0xc92f4e0cbc614cd2046029b847eaff84cbc7ba24","0x5a63264914a1ecb626e32e8ad683704ba7b0621f","0x22a136e7c2713b2df3942fde3fc1ebe08e035968","0x940dbd89e5e2fa0f0a4ebfddea8e5aaa37d6c96a","0xb0e94fcfeff79fd6ee5e02516ecb407343fb92d6","0xa0ee44890c508cd112ba7677c7c144128af38277","0xce4c0752125b20c5c9adce829c26a44235df9dce","0xa0ada0dc2e8a23b209b18ef5b2a45e962c51bdcf","0x2deaa0e8176ba60dfcf106f38a565af9335d6eac","0xe87cd5a662220564ec27509aeea24030bb296003","0x6cab40f3414263a61f9462ccc64902ce69d9ce49","0x12e333f91e8b1b30f8f528e1c306882122e677a7","0x00ca3ae5d1195aed967f44fc706b5db6deff3b67","0x2feff917416cc018a8d2d98aba2d3323fe7d19fb","0x67a4c0d0b0283766fcdd34b25678b5b83ca468a9"]

module.exports = async function(callback) {
  try {
    if (!process.argv[4]) {
      throw new Error('Expected the Ethereum address of the Gateway')
    }
    if (!process.argv[5]) {
      throw new Error('Expected an Ethereum address')
    }
    const gatewayAddr = process.argv[4]
    const instance = await Gateway.at(gatewayAddr)

    let nonces = await Promise.all(candidates.map(async (candidate) => {
      return {
        address: candidate,
        nonce: await instance.nonces(candidate)
      }
    }))

    let validNonces = nonces.filter((x) => x.nonce > 0)
    console.log(JSON.stringify(validNonces))
  } catch (err) {
    callback(err)
  }
  callback()
}

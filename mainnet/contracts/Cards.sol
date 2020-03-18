pragma solidity ^0.4.24;

import "openzeppelin-solidity/contracts/token/ERC721/ERC721Token.sol";


contract CryptoCards is ERC721Token("CryptoCards", "CCC") {

    mapping(address => bool) private registered;

    address owner;
    address public gateway;

    constructor (address _gateway) public {
        owner = msg.sender;
        gateway = _gateway;
    }

    function mintTokens(address _to) external {
        require(msg.sender == owner);
        for (int j = 0; j < 5 ; j++) {
            uint256 tokenId = allTokens.length + 1;
            _mint(_to, tokenId);
        }
    }
    
    function depositToGateway(uint tokenId) public {
        safeTransferFrom(msg.sender, gateway, tokenId);
    }

}

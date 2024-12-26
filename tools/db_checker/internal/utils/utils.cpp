#include "utils.h"

#include <etl/NFTHelpers.hpp>
#include <ripple/basics/Slice.h>
#include <ripple/basics/base_uint.h>
#include <ripple/protocol/AccountID.h>
#include <ripple/protocol/LedgerHeader.h>
#include <ripple/protocol/SField.h>
#include <ripple/protocol/STObject.h>
#include <ripple/protocol/STTx.h>
#include <ripple/protocol/Serializer.h>
#include <ripple/protocol/TxMeta.h>
#include <ripple/protocol/nft.h>

#include <cstddef>
#include <cstdint>
#include <cstring>

void
GetStatesHashFromLedgerHeader(char* ledgerHeaderBlob, int size, char* hash)
{
    ripple::Slice slice{ledgerHeaderBlob, (size_t)size};
    ripple::LedgerHeader info = ripple::deserializeHeader(slice);
    ripple::uint256 h = info.accountHash;
    std::memcpy(hash, h.data(), ripple::uint256::size());
}

void
GetTxHashFromLedgerHeader(char* ledgerHeaderBlob, int size, char* hash)
{
    ripple::Slice slice{ledgerHeaderBlob, (size_t)size};
    ripple::LedgerHeader info = ripple::deserializeHeader(slice);
    ripple::uint256 h = info.txHash;
    std::memcpy(hash, h.data(), ripple::uint256::size());
}

void
GetLedgerHashFromLedgerHeader(char* ledgerHeaderBlob, int size, char* hash)
{
    ripple::Slice slice{ledgerHeaderBlob, (size_t)size};
    ripple::LedgerHeader info = ripple::deserializeHeader(slice, true);
    ripple::uint256 h = info.hash;
    std::memcpy(hash, h.data(), ripple::uint256::size());
}

void
GetAccountTxnIDFromTx(
    char* txBlob,
    unsigned int txSize,
    char* metaBlob,
    unsigned int metaSize,
    unsigned int maxAccount,
    char* accounts,
    unsigned int* accountCount,
    unsigned int* txnIndex
)
{
    try {
        ripple::Slice slice{txBlob, (size_t)txSize};
        ripple::STTx txn(slice);
        ripple::SerialIter meta{metaBlob, (size_t)metaSize};
        // no need real ledger index, just set to 0
        ripple::TxMeta txMeta{txn.getTransactionID(), 0, ripple::STObject(meta, ripple::sfMetadata)};
        *txnIndex = txMeta.getIndex();
        auto affectedAccounts = txMeta.getAffectedAccounts();
        uint32_t i = 0;
        for (auto const& account : affectedAccounts) {
            std::memcpy(&accounts[i * ripple::AccountID::size()], account.data(), ripple::AccountID::size());
            i++;
            if (i >= maxAccount) {
                break;
            }
        }
        *accountCount = i;
    } catch (std::runtime_error const& e) {
        std::cerr << "Exception: " << e.what() << std::endl;
        *accountCount = 0;
        *txnIndex = 0;
    }
}

void
GetNFTFromTx(
    char* txBlob,
    unsigned int txSize,
    char* metaBlob,
    unsigned int metaSize,
    unsigned int maxCount,
    unsigned int* count,
    unsigned int* txIdxs,
    char* nftTokenIds,
    char* tokenChanged,
    char* nftChangedId,
    char* issuer,
    char* uriExists,
    unsigned int* uriSize,
    char* isBurned,
    unsigned int* taxon
)
{
    try{
        ripple::Slice slice{txBlob, (size_t)txSize};
        ripple::STTx txn(slice);
        ripple::SerialIter meta{metaBlob, (size_t)metaSize};
        // no need real ledger index, just set to 0
        ripple::TxMeta txMeta{txn.getTransactionID(), 0, ripple::STObject(meta, ripple::sfMetadata)};
        auto [nftTxData, maybeNft] = etl::getNFTDataFromTx(txMeta, txn);
        *count = nftTxData.size();
        if (*count > maxCount) {
            *count = maxCount;
        }
        for (unsigned int i = 0; i < *count; i++) {
            *txIdxs = nftTxData[i].transactionIndex;
            std::memcpy(&nftTokenIds[i * ripple::uint256::size()], nftTxData[i].tokenID.data(), ripple::uint256::size());
        }
        *tokenChanged = maybeNft.has_value() ? 1 : 0;
        if (maybeNft) {
            std::memcpy(nftChangedId, maybeNft->tokenID.data(), ripple::uint256::size());
            auto const nftIssuer = ripple::nft::getIssuer(maybeNft->tokenID);
            std::memcpy(issuer, nftIssuer.data(), ripple::AccountID::size());
            *uriExists = maybeNft->uri.has_value() ? 1 : 0;
            if (maybeNft->uri) {
                *uriSize = maybeNft->uri->size();
            }
            *isBurned = maybeNft->isBurned ? 1 : 0;
            *taxon = static_cast<uint32_t>(ripple::nft::getTaxon(maybeNft->tokenID));
        }
    }catch (std::exception& e) {
        std::cerr << "Exception: " << e.what() << std::endl;
    }
}

void
GetUriFromTx(char* txBlob, unsigned int txSize, char* metaBlob, unsigned int metaSize, char* uri)
{
    ripple::Slice slice{txBlob, (size_t)txSize};
    ripple::STTx txn(slice);
    ripple::SerialIter meta{metaBlob, (size_t)metaSize};
    ripple::TxMeta txMeta{txn.getTransactionID(), 0, ripple::STObject(meta, ripple::sfMetadata)};
    auto [nftTxData, maybeNft] = etl::getNFTDataFromTx(txMeta, txn);
    if (maybeNft && maybeNft->uri) {
        std::memcpy(uri, maybeNft->uri->data(), maybeNft->uri->size());
    }
}

void
GetDiffFromTx(
    char* txBlob,
    unsigned int txSize,
    char* metaBlob,
    unsigned int metaSize,
    unsigned int maxNode,
    unsigned int* createdNum,
    char* createdIndexes,
    unsigned int* deletedNum,
    char* deletedIndexes,
    unsigned int* updatedNum,
    char* updatedIndexes
)
{
    ripple::Slice slice{txBlob, (size_t)txSize};
    ripple::STTx txn(slice);
    ripple::SerialIter meta{metaBlob, (size_t)metaSize};
    ripple::TxMeta txMeta{txn.getTransactionID(), 0, ripple::STObject(meta, ripple::sfMetadata)};

    *createdNum = 0;
    *deletedNum = 0;
    *updatedNum = 0;
    for (auto const& node : txMeta.getNodes()) {
        auto const index = node.getFieldH256(ripple::sfLedgerIndex);
        if ((*createdNum) < maxNode && node.getFName() == ripple::sfCreatedNode) {
            std::memcpy(&createdIndexes[*createdNum * ripple::uint256::size()], index.data(), ripple::uint256::size());
            (*createdNum)++;
        } else if ((*deletedNum) < maxNode && node.getFName() == ripple::sfDeletedNode) {
            std::memcpy(&deletedIndexes[*deletedNum * ripple::uint256::size()], index.data(), ripple::uint256::size());
            (*deletedNum)++;
        } else if ((*updatedNum) < maxNode && node.getFName() == ripple::sfModifiedNode) {
            std::memcpy(&updatedIndexes[*updatedNum * ripple::uint256::size()], index.data(), ripple::uint256::size());
            (*updatedNum)++;
        }
    }
}

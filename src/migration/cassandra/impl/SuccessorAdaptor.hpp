//------------------------------------------------------------------------------
/*
    This file is part of clio: https://github.com/XRPLF/clio
    Copyright (c) 2025, the clio developers.

    Permission to use, copy, modify, and distribute this software for any
    purpose with or without fee is hereby granted, provided that the above
    copyright notice and this permission notice appear in all copies.

    THE  SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
    WITH  REGARD  TO  THIS  SOFTWARE  INCLUDING  ALL  IMPLIED  WARRANTIES  OF
    MERCHANTABILITY  AND  FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
    ANY  SPECIAL,  DIRECT,  INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
    WHATSOEVER  RESULTING  FROM  LOSS  OF USE, DATA OR PROFITS, WHETHER IN AN
    ACTION  OF  CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
    OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/
//==============================================================================

#pragma once

#include "migration/cassandra/CassandraMigrationBackend.hpp"
#include "migration/cassandra/impl/FullTableScannerAdapterBase.hpp"

#include <xrpl/basics/Blob.h>
#include <xrpl/basics/base_uint.h>

#include <cstdint>
#include <functional>
#include <memory>
#include <string>
#include <tuple>
#include <utility>

namespace migration::cassandra::impl {

struct SuccessorTable {
    // key, seq, next
    using Row = std::tuple<ripple::Blob, std::uint32_t, ripple::Blob>;
    static constexpr char const* kPARTITION_KEY = "key";
    static constexpr char const* kTABLE_NAME = "successor";
};

/**
 * @brief Adaptor for the successor table.
 */
class SuccessorAdapter : public impl::FullTableScannerAdapterBase<SuccessorTable> {
public:
    using OnSuccessorRead = std::function<void(std::string const& key, std::uint32_t seq, std::string const& next)>;

    /**
     * @brief Construct a new Transactions Adapter object
     *
     * @param backend The backend
     * @param onTxRead The callback to call when a transaction is read
     */
    explicit SuccessorAdapter(std::shared_ptr<CassandraMigrationBackend> backend, OnSuccessorRead onSuccssorRead)
        : FullTableScannerAdapterBase<SuccessorTable>(backend), onSuccessorRead_{std::move(onSuccssorRead)}
    {
    }

    /**
     * @brief Called when a row is read
     *
     * @param row The row to read
     */
    void
    onRowRead(SuccessorTable::Row const& row) override;

private:
    OnSuccessorRead onSuccessorRead_;
};

}  // namespace migration::cassandra::impl

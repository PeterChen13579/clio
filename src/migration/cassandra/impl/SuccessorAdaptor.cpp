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

#include "migration/cassandra/impl/SuccessorAdaptor.hpp"

#include "util/Assert.hpp"

#include <string>

namespace migration::cassandra::impl {

void
SuccessorAdapter::onRowRead(SuccessorTable::Row const& row)
{
    auto const& [key, ledgerSeq, next] = row;
    ASSERT(!key.empty() && !next.empty(), "key and next blob can not be empty in successor table");
    auto const k = std::string(std::begin(key), std::end(key));
    auto const n = std::string(std::begin(next), std::end(next));
    onSuccessorRead_(k, ledgerSeq, n);
}

}  // namespace migration::cassandra::impl

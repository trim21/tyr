// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>

package null_test

import (
	"encoding/json"
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/stretchr/testify/require"

	"tyr/internal/pkg/null"
)

func TestNull_Ptr(t *testing.T) {
	t.Parallel()

	n := null.Int{Set: true, Value: 1}
	require.Equal(t, 1, *n.Ptr())

	n = null.Int{Set: false, Value: 1}
	require.Nil(t, n.Ptr())
}

func TestNull_Default(t *testing.T) {
	t.Parallel()

	n := null.Int{Set: true, Value: 1}
	require.Equal(t, 1, n.Default(10))

	n = null.Int{Set: false, Value: 1}
	require.Equal(t, 10, n.Default(10))
}

func TestNull_Interface(t *testing.T) {
	t.Parallel()

	n := null.Int{Set: true, Value: 1}
	require.EqualValues(t, 1, n.Interface())

	n = null.Int{Set: false, Value: 1}
	require.EqualValues(t, nil, n.Interface())
}

func TestNull_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	var n null.Int
	require.NoError(t, json.Unmarshal([]byte("10"), &n))
	require.EqualValues(t, 10, n.Value)

	n = null.Int{}
	require.NoError(t, json.Unmarshal([]byte(" null "), &n))
	require.False(t, n.Set)
}

func Test_UnmarshalBencode(t *testing.T) {
	t.Parallel()

	var n null.Int
	require.NoError(t, bencode.Unmarshal([]byte("i10e"), &n))
	require.EqualValues(t, 10, n.Value)

	var s struct {
		N null.Int `bencode:"n"`
	}
	require.NoError(t, bencode.Unmarshal([]byte("de"), &s))
	require.False(t, s.N.Set)

	var s2 struct {
		N null.Null[bencode.Bytes] `bencode:"n"`
	}
	require.NoError(t, bencode.Unmarshal([]byte("de"), &s2))
	require.False(t, s2.N.Set)

	var s3 struct {
		N null.Null[bencode.Bytes] `bencode:"n"`
	}
	require.NoError(t, bencode.Unmarshal([]byte("d1:ni10ee"), &s3))
	require.True(t, s3.N.Set)
	require.EqualValues(t, bencode.Bytes("i10e"), s3.N.Value)
}

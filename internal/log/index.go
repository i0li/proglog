package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

const (
	offWidth uint64 = 4
	posWidth uint64 = 8
	endWidth        = offWidth + posWidth
)

type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}
	// ファイル情報の取得
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	// sizeは次に書き込む際のファイル内の場所を示すことにもなる
	idx.size = uint64(fi.Size())
	// fのサイズをc.Segment.MaxIndexBytesで指定したサイズにする
	// 一度マップされたファイルがサイズを変更できないため最大サイズにしている
	if err = os.Truncate(
		f.Name(), int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, err
	}
	// 第1引数:file.Fd()はファイルのディスクリプタ（OSがファイルを識別するために使用する整数）を取得する
	// 第2引数:マッピングされたメモリに対するアクセス権を設定。（読み取りと書き込み）
	// 第3引数:ファイルが変更された場合、他のプロセスでも変更が確認できる(MAP_PRIVATEにすると他のプロセスでは変更が確認できない)
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}
	return idx, nil
}

func (i *index) Close() error {
	// メモリにマップされたファイルのデータを即座にディスク上のファイルと同期する
	// 厳密にはOSのファイルシステムバッファに書き込まれる
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}
	// ファイルシステムバッファ内のデータを確実にストレージに保存する
	if err := i.file.Sync(); err != nil {
		return err
	}
	// 実際のデータ量まで切り詰める
	// 0埋めされた使われていない領域が存在しないようにするため（次の起動のため）
	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	return i.file.Close()
}

func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		// End of File
		return 0, 0, io.EOF
	}
	// 最新のエントリを取得するため
	if in == -1 {
		out = uint32((i.size / endWidth) - 1)
	} else {
		out = uint32(in)
	}
	pos = uint64(out) * endWidth
	if i.size < pos+endWidth {
		return 0, 0, io.EOF
	}
	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+endWidth])
	return out, pos, nil
}

func (i *index) Write(off uint32, pos uint64) error {
	if i.isMaxed() {
		return io.EOF
	}
	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+endWidth], pos)
	i.size += uint64(endWidth)
	return nil
}

func (i *index) isMaxed() bool {
	return uint64(len(i.mmap)) < i.size+endWidth
}

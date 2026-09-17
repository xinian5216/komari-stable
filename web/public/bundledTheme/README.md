# web/public/bundledTheme

构建期生成物目录（**`next.zip` 不入库**），由 `scripts/prepare-assets.py theme` 按
`bundled-themes.lock.json` 下载并校验后写入。

| 文件 | 说明 |
| --- | --- |
| `next.zip` | 打包进二进制的 Komari Next 主题包（`//go:embed bundledTheme`），只在**全新安装**时 seed 到 `data/theme/next/` |
| `../assets.provenance.json` | 记录两个构建期资产的来源（repository / tag / commit / asset / sha256） |

本文件（`README.md`）随仓库提交，用于在没有生成物时也能满足 `//go:embed bundledTheme`
的编译前提；运行时若发现 `next.zip` 不存在会记录 warning 并跳过 seed（安装仍成功，`theme` 保持默认）。

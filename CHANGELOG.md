# Changelog

## [0.1.3](https://github.com/MohammadBnei/gostreampuller/compare/0.1.2...0.1.3) (2025-07-11)

## [0.1.2](https://github.com/MohammadBnei/gostreampuller/compare/0.1.1...0.1.2) (2025-07-11)

## [0.1.1](https://github.com/MohammadBnei/gostreampuller/compare/0.1.0...0.1.1) (2025-07-11)

# 0.1.0 (2025-07-11)


### Bug Fixes

* Align comment for progressID query parameter ([6c97701](https://github.com/MohammadBnei/gostreampuller/commit/6c97701674681b80d4237b28acc3a7a3379de23b))
* Clear video URL input on page load ([e3ea273](https://github.com/MohammadBnei/gostreampuller/commit/e3ea2739259ef2b9b957f8be6c2dd4bce5eafee2))
* Correct receiver type for PlayWebStream method ([38eebe0](https://github.com/MohammadBnei/gostreampuller/commit/38eebe0753ff7ee28a06e0a56f1afd1e0ede8467))
* Correctly check yt-dlp and ffmpeg executable versions ([3654bf9](https://github.com/MohammadBnei/gostreampuller/commit/3654bf91916205e0ef959f1123deb47eaa6c3c14))
* Correctly initialize template data and handle dynamic DOM updates in web UI ([e056c11](https://github.com/MohammadBnei/gostreampuller/commit/e056c11ce21d238dbe13c4f3390239b0f7d65671))
* Correctly parse yt-dlp output for downloaded file paths ([fca481a](https://github.com/MohammadBnei/gostreampuller/commit/fca481a3f9a62a47e7e7b003a171abe56af894b0))
* Ensure StreamVideo pipes through FFmpeg for format conversion ([c56f562](https://github.com/MohammadBnei/gostreampuller/commit/c56f562661b38f0259cab16129e0e5edc52e7895))
* Implement http.Flusher in logging middleware's response recorder ([50aad9f](https://github.com/MohammadBnei/gostreampuller/commit/50aad9fc75d277dad688203691f23837a28c5916))
* Improve error message for unsupported SSE streaming ([8b2b5c2](https://github.com/MohammadBnei/gostreampuller/commit/8b2b5c22efab5dc382167610aa144f12999eea7a))
* Log error when streaming is unsupported ([c287968](https://github.com/MohammadBnei/gostreampuller/commit/c2879688121ca2a913f3af62c7db779d0d51485c))
* Make health endpoint method-specific to resolve routing conflict ([614ef14](https://github.com/MohammadBnei/gostreampuller/commit/614ef146a56a68f0866d1f80c0d01fce507a35c3))
* Pass empty progressID to API-facing download/stream handlers ([0792ced](https://github.com/MohammadBnei/gostreampuller/commit/0792ced0d9f7d2f76189128caedcf924a3d79753))
* Prefix web stream routes to resolve pattern conflicts ([19374a1](https://github.com/MohammadBnei/gostreampuller/commit/19374a1c1d9a203126d6aad713751d9e8e68e587))
* Remove TestPipedCommandReadCloserClose due to refactor ([8fec155](https://github.com/MohammadBnei/gostreampuller/commit/8fec155dbaaed84568e750c1564fd9e00dd2ba39))
* Use video title for temporary file name ([a88c5c4](https://github.com/MohammadBnei/gostreampuller/commit/a88c5c4a841d90fbd44d92423b5f9b1b906de53e))


### Features

* Add 'Load Video Info' button and dynamic UI for web stream ([5a7d64c](https://github.com/MohammadBnei/gostreampuller/commit/5a7d64c6d74e0dc76a5d20639ec83fda7155a359))
* Add audio download to browser and SSE progress tracking ([58093ee](https://github.com/MohammadBnei/gostreampuller/commit/58093eef96fcc812e4094427928cb2ef27e71c78))
* Add audio quality selector and default to 192kbps ([582c960](https://github.com/MohammadBnei/gostreampuller/commit/582c960af9670b815f4f5f5e3667989bc6e35cb5))
* Add context to Downloader methods and use CommandContext ([2e2f503](https://github.com/MohammadBnei/gostreampuller/commit/2e2f503be8b9b8a4d2143a9d8c9cf1958124aca0))
* Add dedicated handlers for download and stream functionalities ([7dfd1d2](https://github.com/MohammadBnei/gostreampuller/commit/7dfd1d275feae7a54c0b5e0c71729c9bcd63bbc6))
* Add direct video/audio download to web UI ([d8e0e8d](https://github.com/MohammadBnei/gostreampuller/commit/d8e0e8d8c8b7ae27b0b5cc87907f6b2603fce4b7))
* Add download buttons to web stream form ([970882b](https://github.com/MohammadBnei/gostreampuller/commit/970882b5266378f3d7968f871e542f68f466d0a5))
* Add DOWNLOAD_DIR config and use it in downloader ([c96885d](https://github.com/MohammadBnei/gostreampuller/commit/c96885d45a72ba1b372cf8646c6273fa22ffdd64))
* Add FFMPEG and YTDLP config paths ([d25543e](https://github.com/MohammadBnei/gostreampuller/commit/d25543e1c173360b491d1a9a9a4f777dd25b66a6))
* Add ProgressManager for real-time event broadcasting ([6919ed3](https://github.com/MohammadBnei/gostreampuller/commit/6919ed3ed716231c4a03cf359d1474107c1c02bd))
* Add stream service for proxying video and audio ([ee2c85d](https://github.com/MohammadBnei/gostreampuller/commit/ee2c85d5934c83bd778f5abd9330d92cfc6bb977))
* Add video/audio streaming and rename file download functions ([e52cc65](https://github.com/MohammadBnei/gostreampuller/commit/e52cc65b55bf146de00911cd5a572f59b8cca45c))
* Add web-based video streaming route with HTML form and player ([dcd1006](https://github.com/MohammadBnei/gostreampuller/commit/dcd1006cc9a86bcf478dea5a8f6c70ac602947d0))
* Add yt-dlp and ffmpeg executable verification to config init ([5311934](https://github.com/MohammadBnei/gostreampuller/commit/53119348f8def0b333f25ba920ea672337a4f492))
* Embed web HTML templates using go:embed ([747b754](https://github.com/MohammadBnei/gostreampuller/commit/747b754923e3b8d8a519a4970b29e406ba37101a))
* Implement multi-step web UI for video operations ([8d932be](https://github.com/MohammadBnei/gostreampuller/commit/8d932beaac69812291ea1f2be94524d53504d552))
* Implement SSE and dynamic UI updates for web stream ([b1554fc](https://github.com/MohammadBnei/gostreampuller/commit/b1554fc1fb3c3a4558c54f8536cc9ee1b6933788))
* Implement SSE for progress tracking in web stream handler ([7be89cd](https://github.com/MohammadBnei/gostreampuller/commit/7be89cdc289036ebbf2cec26f1b9a1dfe08eba58))
* Install yt-dlp and ffmpeg in Dockerfile ([9fca2d1](https://github.com/MohammadBnei/gostreampuller/commit/9fca2d1b441fc8560e2c2af61fbe4dd96dc09b63))
* Integrate ProgressManager into Downloader methods for progress updates ([a99ff8c](https://github.com/MohammadBnei/gostreampuller/commit/a99ff8c23e1d016535a23481180b1eb5fc2e92b0))
* Refactor web UI for two-step operation and consistent progress tracking ([bd1987d](https://github.com/MohammadBnei/gostreampuller/commit/bd1987d63fa93781cccdd719f2403406a3c6f5aa))
* Replace resolution input with a select dropdown in stream.html ([c4dd970](https://github.com/MohammadBnei/gostreampuller/commit/c4dd970195c46b770a4896b6d6b8ae7c67cbdb73))
* Return metadata and use unique filenames for downloads ([a41c76e](https://github.com/MohammadBnei/gostreampuller/commit/a41c76e47ffaee7c355e155b2a596a53623e070e))
* Serve web stream page on root path "/" ([f81e93d](https://github.com/MohammadBnei/gostreampuller/commit/f81e93d693f3c4182226e6692c1caa809012f9bb))
* Use /web prefix for web streaming routes ([ebdf378](https://github.com/MohammadBnei/gostreampuller/commit/ebdf3786c4f9651a5970a06051db9e0c0ed09c29))

# chrome yt-dlp extension

[English](#english) · [Русский](#русский) · [Releases / Релизы](https://github.com/AndreyFulov/Chrome-yt-dlp-extension/releases)

## English

Download YouTube videos and Shorts on Windows or macOS from the Chrome toolbar or the **Download** button to the left of the Like button. Choose MP4 video, M4A audio, or MP3 audio and select from the quality options available for that video. The default is your system Downloads folder. Choose another folder in the download menu to reuse it for future downloads.

### How it works

The Chrome extension sends the video URL and your selection to a local Go helper over [Chrome Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging). The helper runs `yt-dlp` to find and download media, uses `ffmpeg` and `ffprobe` to process it, and saves the result to your computer. Deno runs the YouTube challenge scripts supplied by `yt-dlp-ejs` (EJS). Downloads continue when you dismiss the menu, switch tabs, or minimize Chrome while the browser is running.

The extension first requests formats without cookies. When YouTube requires sign-in, it can retry using the YouTube cookies from the current Chrome profile. Cookies go to the local helper. You can also choose a Netscape-format `cookies.txt` from the menu. [yt-dlp cookie guide](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp)

### Install from GitHub Releases

1. Download and extract a ZIP for your platform from [Releases](https://github.com/AndreyFulov/Chrome-yt-dlp-extension/releases). Compare its SHA-256 with the matching entry in `SHA256SUMS.txt` if you want to verify the download.

   | Platform | Archive |
   | --- | --- |
   | Windows x64 | `ytdlp-chrome-windows-amd64.zip` |
   | macOS Apple Silicon | `ytdlp-chrome-darwin-arm64.zip` |
   | macOS Intel | `ytdlp-chrome-darwin-amd64.zip` |

2. Install `yt-dlp` with EJS, `ffmpeg` and `ffprobe`, and Deno. The release ZIP contains the ready-built helper, the installer, and the Chrome extension. The installer registers the helper for your user account and copies media tools only if you supply them. Put the tools on your `PATH` or in `backend/tools` before running the installer. Use `.exe` filenames on Windows and names without extensions on macOS. [yt-dlp and EJS setup](https://github.com/yt-dlp/yt-dlp/wiki/EJS) · [FFmpeg downloads](https://ffmpeg.org/download.html) · [Deno installation](https://docs.deno.com/runtime/getting_started/installation/)
3. From the extracted release folder, install the helper.

   **Windows (PowerShell)**

   ```powershell
   .\backend\install.ps1
   ```

   **macOS (Terminal)**

   ```sh
   sh backend/install-macos.sh
   ```

4. Open `chrome://extensions`, turn on **Developer mode**, choose **Load unpacked**, and select the extracted `frontend` folder. To switch between English and Russian, open **chrome yt-dlp extension** from the Chrome toolbar and use its language setting. [Chrome guide to loading unpacked extensions](https://developer.chrome.com/docs/extensions/get-started/tutorial/hello-world#load-unpacked)

The helper checks for tools in its installed `tools` directory, then on `PATH`. On macOS it also checks the standard Homebrew binary directories. Go is required for building from source. The same commands build the helper when run from a source checkout with Go 1.22 or newer and no prebuilt helper next to the installer.

The prebuilt macOS helper is not notarized. If Gatekeeper blocks it when Chrome opens the extension, you can build the helper locally from a source checkout instead. [Apple’s guide to Gatekeeper](https://support.apple.com/en-us/102445)

### Update or uninstall

- **Update.** Finish active downloads and quit Chrome. Download the new ZIP for your platform, run its helper installer, and replace the contents of the previously loaded `frontend` folder with the new `frontend` files. Launch Chrome, click **Reload** on the extension card in `chrome://extensions`, and refresh your YouTube tabs.
- **Update media tools.** Update them using the method you installed them with. If you supplied a `backend/tools` directory, replace the binaries there and rerun the installer so it copies the new versions.
- **Uninstall.** Remove the extension in `chrome://extensions`. From the extracted release folder, run `.\backend\install.ps1 -Uninstall` on Windows or `sh backend/install-macos.sh --uninstall` on macOS to remove the native helper and its registration.

### Build and license

Run `go test ./...` and `go vet ./...` from `backend`. Run `pwsh -File backend/package.ps1` from the project root to build the platform ZIPs locally. The release workflow builds and publishes assets when a matching `vX.Y.Z` tag is pushed, using the extension version in `frontend/manifest.json`.

The extension and native helper are distributed under the [MIT License](LICENSE). Third-party tools have their own licenses. Download only material that you have permission to save.

## Русский

Скачивайте ролики и Shorts на Windows и macOS через значок в Chrome или кнопку **«Скачать»** слева от лайков. Выберите MP4 для видео, M4A или MP3 для аудио и подходящее качество из доступных вариантов. По умолчанию файлы сохраняются в системную папку «Загрузки». Папку можно изменить в меню скачивания, и выбор сохранится для следующих загрузок.

### Как это работает

Расширение передаёт адрес видео и выбранные параметры локальному помощнику на Go через [Chrome Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging). Помощник запускает `yt-dlp` для поиска и загрузки потоков, использует `ffmpeg` и `ffprobe` для обработки и сохраняет результат на компьютер. Deno запускает скрипты `yt-dlp-ejs` (EJS), которые нужны для обработки JavaScript-проверок YouTube. Загрузка продолжается после закрытия меню, переключения вкладки или сворачивания окна, пока Chrome запущен.

Сначала расширение запрашивает форматы без cookies. Если YouTube требует вход, расширение может повторить запрос с cookies YouTube из текущего профиля Chrome. Cookies передаются локальному помощнику. При необходимости можно выбрать файл `cookies.txt` в формате Netscape. [Документация yt-dlp о cookies](https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp)

### Установка из GitHub Releases

1. Скачайте архив для своей системы со страницы [Releases](https://github.com/AndreyFulov/Chrome-yt-dlp-extension/releases) и распакуйте его. Для проверки файла сравните его SHA-256 со значением из `SHA256SUMS.txt`.

   | Платформа | Архив |
   | --- | --- |
   | Windows x64 | `ytdlp-chrome-windows-amd64.zip` |
   | macOS Apple Silicon | `ytdlp-chrome-darwin-arm64.zip` |
   | macOS Intel | `ytdlp-chrome-darwin-amd64.zip` |

2. Установите `yt-dlp` с EJS, `ffmpeg` и `ffprobe`, а также Deno. В релизный архив входят готовый помощник, установщик и расширение Chrome. Установщик регистрирует помощник для текущего пользователя и копирует инструменты, если вы их предоставили. Добавьте инструменты в `PATH` или положите их в `backend/tools` перед установкой. На Windows файлы должны иметь расширение `.exe`, на macOS — имена без расширения. [Настройка yt-dlp и EJS](https://github.com/yt-dlp/yt-dlp/wiki/EJS) · [Сборки FFmpeg](https://ffmpeg.org/download.html) · [Установка Deno](https://docs.deno.com/runtime/getting_started/installation/)
3. Запустите установщик помощника из папки распакованного архива.

   **Windows (PowerShell)**

   ```powershell
   .\backend\install.ps1
   ```

   **macOS (Терминал)**

   ```sh
   sh backend/install-macos.sh
   ```

4. Откройте `chrome://extensions`, включите **«Режим разработчика»**, нажмите **«Загрузить распакованное расширение»** и укажите папку `frontend`. Чтобы переключаться между русским и английским, откройте **chrome yt-dlp extension** на панели Chrome и выберите язык в настройках. [Инструкция Chrome по загрузке распакованного расширения](https://developer.chrome.com/docs/extensions/get-started/tutorial/hello-world#load-unpacked)

Помощник ищет инструменты в установленной папке `tools`, затем в `PATH`. На macOS он также проверяет стандартные каталоги Homebrew. Для сборки из исходников нужен Go 1.22 или новее. Те же команды соберут помощник при запуске из копии исходного кода, если рядом с установщиком нет готового файла помощника.

Готовый помощник для macOS не нотаризован. Если Gatekeeper блокирует его при открытии расширения в Chrome, можно собрать помощник локально из исходников. [Справка Apple о Gatekeeper](https://support.apple.com/en-us/102445)

### Обновление и удаление

- **Обновление.** Дождитесь завершения активных загрузок и выйдите из Chrome. Скачайте новый архив, запустите установщик помощника и замените содержимое ранее загруженной папки `frontend` новыми файлами. Запустите Chrome, нажмите **«Обновить»** на карточке расширения в `chrome://extensions` и перезагрузите вкладки YouTube.
- **Обновление инструментов.** Используйте тот же способ, которым их установили. Если вы добавляли файлы в `backend/tools`, замените их там и повторно запустите установщик, чтобы он скопировал новые версии.
- **Удаление.** Удалите расширение в `chrome://extensions`. Из папки распакованного архива выполните `.\backend\install.ps1 -Uninstall` на Windows или `sh backend/install-macos.sh --uninstall` на macOS, чтобы удалить помощник и его регистрацию.

### Сборка и лицензия

Выполните `go test ./...` и `go vet ./...` из папки `backend`. Для локальной сборки архивов выполните `pwsh -File backend/package.ps1` из корня проекта. При отправке тега `vX.Y.Z`, соответствующего версии в `frontend/manifest.json`, GitHub Actions собирает и публикует архивы релиза.

Расширение и локальный помощник распространяются по [лицензии MIT](LICENSE). У сторонних инструментов свои лицензии. Скачивайте материалы, которые вам разрешено сохранять.

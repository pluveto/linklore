# Linklore

Linklore is a util that allows you to export a note from an Obsidian note library and parse all the internal links, replacing them with real links.

This is very useful when you want to share your notes. This program can help you convert the links in your notes to real links relative to your Obsidian publish site, so that you can use them elsewhere.

## Usage

The program can be executed using the following command:

```shell
linklore -i <input file> [-d <dir>] [-o <output file>] [-p <prefix>] [-f]
```

The available options are:

- `-i <input file>`: Specifies the input file to be processed.
- `-d <dir>`: Specifies the directory where the program will scan for files. (Default: current directory)
- `-o <output file>`: Specifies the output file where the processed content will be saved. (Default: `<input file basename> + .out.md`)
- `-p <prefix>`: Sets the prefix for the real links. (Default: `/`)
- `-f`: Forces the program to overwrite the output file if it already exists.

You can also set these options using a `.env` file or environment variables:

- `LINKLORE_INPUT_FILE`
- `LINKLORE_OUTPUT_FILE`
- `LINKLORE_BASE_DIR`
- `LINKLORE_PREFIX` or `LINKLORE_BASE_URL`
- `LINKLORE_FORCE`

## How it works

The program follows these steps to process the input file:

1. Build an index:
   - The program scans all files (not just `.md` files) in the specified directory (`dir`) and creates an index that records the path and filename of each file.
   - Each file is identified by a unique key, which is the filename without the extension. For example, the key for `foo/bar.md` would be `bar`.
   - The index also includes other information about each file, such as the name, basename, extension, and path relative to the directory (`dir`).
   - If the number of files exceeds 10,000, an error is reported, as the program currently does not support such a large number of files.
2. Read the input file and parse the links:
   - The program uses regular expressions to parse the links in the input file.
   - There are several possible link formats, including:
     - `![[hello.png]]`: Replaced with the real link `![hello](prefix+path)`. Only this format allows file extensions.
     - `[[hello]]`: Replaced with the real link `[hello](prefix+path)`.
     - `[[hello|world]]`: Replaced with the real link `[world](prefix+path)`.
     - `[[hello^world]]`: Treated the same as format 2 and replaced with `[hello](prefix+path)`.
     - `[[hello#world]]`: Replaced with the real link `[hello](prefix+path#world)`.
   - If a link does not match any file in the index, an error is reported. The program continues processing to find all errors.
3. The processed content is written to the output file without overwriting the original file. If the output file already exists, an error is reported unless the `-f` option is specified.

## Installation

You can download the program from the [releases page](https://github.com/pluveto/linklore/releases).

Alternatively, you can build it from source:

1. Make sure you have Go installed on your system. If not, you can download it from the [official Go website](https://golang.org/dl/).
2. Ensure that you have the `make` command installed.
3. Build the program by running the following command:

```shell
make build
```

The program artifacts will be placed in the `./dist` directory.

## License

Licensed under the [MIT License](LICENSE).

return {
  {
    "nvim-treesitter/nvim-treesitter",
    branch = "main",
    lazy = false,
    priority = 500,
    build = ":TSUpdate",
    config = function()
      local parsers = {
        "lua",
        "vim",
        "vimdoc",
        "markdown",
        "markdown_inline",
        "bash",
        "dockerfile",
        "make",
        "json",
        "yaml",
        "typescript",
        "tsx",
        "javascript",
      }

      require("nvim-treesitter").install(parsers)

      local filetypes = {
        "lua",
        "vim",
        "help",
        "markdown",
        "markdown_inline",
        "bash",
        "sh",
        "dockerfile",
        "make",
        "json",
        "yaml",
        "typescript",
        "typescriptreact",
        "javascript",
        "javascriptreact",
      }

      vim.api.nvim_create_autocmd("FileType", {
        pattern = filetypes,
        callback = function(args)
          pcall(vim.treesitter.start, args.buf)
          vim.bo[args.buf].indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
        end,
      })
    end,
  },
}

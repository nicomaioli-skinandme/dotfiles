return {
  {
    "mfussenegger/nvim-lint",
    event = { "BufReadPre", "BufNewFile" },
    config = function()
      local lint = require("lint")
      lint.linters_by_ft = {
        dockerfile = { "hadolint" },
      }

      local group = vim.api.nvim_create_augroup("nvim-lint", { clear = true })
      vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost", "InsertLeave" }, {
        group = group,
        callback = function()
          lint.try_lint()
        end,
      })
    end,
  },
}

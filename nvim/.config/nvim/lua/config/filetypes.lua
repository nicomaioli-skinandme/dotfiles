vim.filetype.add({
	filename = {
		["Dockerfile"] = "dockerfile",
	},
	pattern = {
		["[Dd]ockerfile.*"] = "dockerfile",
		[".*/docker%-compose%.ya?ml"] = "yaml.docker-compose",
		[".*%.mk"] = "make",
	},
})

// Match Tailwind arbitrary values, not standalone colors used by the accent picker.
const ARBITRARY_COLOR = /(?:^|:)!?(?:[a-z][\w-]*-\[|\[(?:--)?[\w-]+:)([^\]\s]+)\]/gi;
const RAW_COLOR =
	/#[\da-f]{3,8}(?![\da-f])|(?:rgba?|hsla?|hwb|oklch|oklab|lch|lab|color|color-mix)\(/i;

export default {
	meta: { name: "cratebug" },
	rules: {
		"no-raw-tailwind-colors": {
			meta: {
				type: "problem",
				schema: [],
				messages: {
					token: "Use a mapped theme utility instead of a raw color in '{{className}}'.",
				},
			},
			create(context) {
				function check(node, value) {
					if (typeof value !== "string") return;
					for (const className of value.split(/\s+/)) {
						for (const match of className.matchAll(ARBITRARY_COLOR)) {
							// SVG fragment URLs and arbitrary selector variants aren't colors.
							if (
								match[1].includes("url(") ||
								className[match.index + match[0].length] === ":"
							)
								continue;
							if (RAW_COLOR.test(match[1])) {
								context.report({ node, messageId: "token", data: { className } });
								break;
							}
						}
					}
				}
				return {
					Literal(node) {
						check(node, node.value);
					},
					TemplateElement(node) {
						check(node, node.value.cooked);
					},
				};
			},
		},
	},
};

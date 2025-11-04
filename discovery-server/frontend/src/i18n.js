import { createI18n } from 'vue-i18n';

async function loadLocaleMessages() {
    const locales = import.meta.glob('../../locales/*.json');
    const messages = {};
    for (const path in locales) {
        const matched = path.match(/([A-Za-z0-9-_]+)\.json$/i);
        if (matched && matched.length > 1) {
            const locale = matched[1];
            const module = await locales[path]();
            messages[locale] = module.default;
        }
    }
    return messages;
}

export default async function setupI18n() {
    const messages = await loadLocaleMessages();
    return createI18n({
        legacy: false,
        locale: navigator.language.split('-')[0] || 'en',
        fallbackLocale: 'en',
        messages,
    });
}

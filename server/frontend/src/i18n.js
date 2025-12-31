import { createI18n } from 'vue-i18n';

async function loadLocaleMessages() {
    const messages = {};
    for (const locale of ['en', 'zh']) {
        try {
            const response = await fetch(`/locales/${locale}.json`);
            if (response.ok) {
                messages[locale] = await response.json();
            }
        } catch (e) {
            console.error(`Failed to load locale ${locale}`, e);
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

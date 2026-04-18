import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FormDecoder from './FormDecoder.vue'

describe('FormDecoder.vue', () => {
  it('correctly decodes www-form-urlencoded data', async () => {
    const wrapper = mount(FormDecoder)
    const textarea = wrapper.find('textarea')
    
    await textarea.setValue('name=John+Doe&age=25&hobby=coding&hobby=music')
    
    // Check decoded data in table
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(4)
    expect(rows[0].text()).toContain('name')
    expect(rows[0].text()).toContain('John Doe')
    expect(rows[2].text()).toContain('hobby')
    expect(rows[2].text()).toContain('coding')
    expect(rows[3].text()).toContain('music')
  })

  it('generates correct JSON output including multi-value fields', async () => {
    const wrapper = mount(FormDecoder)
    const textarea = wrapper.find('textarea')
    
    await textarea.setValue('key1=val1&key2=val2&key1=val3')
    
    // Trigger "View as JSON"
    const buttons = wrapper.findAll('button')
    const viewJsonBtn = buttons.find(b => b.text().includes('View as JSON'))
    await viewJsonBtn?.trigger('click')
    
    const pre = wrapper.find('pre')
    const json = JSON.parse(pre.text())
    
    expect(json.key1).toEqual(['val1', 'val3'])
    expect(json.key2).toBe('val2')
  })

  it('filters decoded data based on search term', async () => {
    const wrapper = mount(FormDecoder)
    await wrapper.find('textarea').setValue('apple=red&banana=yellow&cherry=red')
    
    const searchInput = wrapper.find('input[placeholder="Search by field name"]')
    await searchInput.setValue('ba')
    
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('banana')
  })
})

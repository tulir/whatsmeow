package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// Este arquivo contém exemplos de uso das funções helper para botões, listas e mensagens interativas

// Example1_SimpleButtons demonstra como enviar uma mensagem com botões simples
func Example1_SimpleButtons(cli *whatsmeow.Client, targetJID types.JID) {
	// Criar botões
	buttons := []*waE2E.ButtonsMessage_Button{
		{
			ButtonID: proto.String("btn_yes"),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String("✅ Sim"),
			},
			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
		},
		{
			ButtonID: proto.String("btn_no"),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String("❌ Não"),
			},
			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
		},
	}

	// Construir mensagem
	msg := cli.BuildButtonsMessage(
		"Você gostou deste produto?",
		"Sua opinião é importante para nós",
		buttons,
		nil, // contextInfo
	)

	// Enviar mensagem
	resp, err := cli.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		log.Printf("Erro ao enviar mensagem com botões: %v\n", err)
	} else {
		fmt.Printf("Mensagem com botões enviada! ID: %s\n", resp.ID)
	}
}

// Example2_ListMessage demonstra como enviar uma mensagem com lista de opções
func Example2_ListMessage(cli *whatsmeow.Client, targetJID types.JID) {
	// Criar seções da lista
	sections := []*waE2E.ListMessage_Section{
		{
			Title: proto.String("🍕 Pizzas"),
			Rows: []*waE2E.ListMessage_Row{
				{
					RowID:       proto.String("pizza_margherita"),
					Title:       proto.String("Margherita"),
					Description: proto.String("Tomate, mozzarella e manjericão"),
				},
				{
					RowID:       proto.String("pizza_pepperoni"),
					Title:       proto.String("Pepperoni"),
					Description: proto.String("Mozzarella e pepperoni"),
				},
			},
		},
		{
			Title: proto.String("🍔 Hambúrgueres"),
			Rows: []*waE2E.ListMessage_Row{
				{
					RowID:       proto.String("burger_classic"),
					Title:       proto.String("Clássico"),
					Description: proto.String("Hambúrguer simples com alface e tomate"),
				},
				{
					RowID:       proto.String("burger_bacon"),
					Title:       proto.String("Bacon"),
					Description: proto.String("Com bacon crocante"),
				},
			},
		},
	}

	// Construir mensagem
	msg := cli.BuildListMessage(
		"Cardápio do Dia",
		"Escolha o que deseja pedir:",
		"📋 Ver Cardápio",
		sections,
		"Toque no botão para ver as opções",
	)

	// Enviar mensagem
	resp, err := cli.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		log.Printf("Erro ao enviar lista: %v\n", err)
	} else {
		fmt.Printf("Lista enviada! ID: %s\n", resp.ID)
	}
}

// Example3_TemplateButtons demonstra como usar botões de template (hydrated buttons)
func Example3_TemplateButtons(cli *whatsmeow.Client, targetJID types.JID) {
	// Criar botões de template
	buttons := []*waE2E.HydratedTemplateButton{
		{
			HydratedButton: &waE2E.HydratedTemplateButton_QuickReplyButton{
				QuickReplyButton: &waE2E.HydratedTemplateButton_HydratedQuickReplyButton{
					DisplayText: proto.String("Ver Catálogo"),
					ID:          proto.String("view_catalog"),
				},
			},
		},
		{
			HydratedButton: &waE2E.HydratedTemplateButton_UrlButton{
				UrlButton: &waE2E.HydratedTemplateButton_HydratedURLButton{
					DisplayText: proto.String("🌐 Visitar Site"),
					URL:         proto.String("https://exemplo.com"),
				},
			},
		},
		{
			HydratedButton: &waE2E.HydratedTemplateButton_CallButton{
				CallButton: &waE2E.HydratedTemplateButton_HydratedCallButton{
					DisplayText: proto.String("📞 Ligar"),
					PhoneNumber: proto.String("+5511999999999"),
				},
			},
		},
	}

	// Construir mensagem
	msg := cli.BuildTemplateButtonsMessage(
		"Bem-vindo à nossa loja! 🛍️",
		"Atendimento: Segunda a Sexta, 9h-18h",
		buttons,
		nil,
	)

	// Enviar mensagem
	resp, err := cli.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		log.Printf("Erro ao enviar template buttons: %v\n", err)
	} else {
		fmt.Printf("Template buttons enviado! ID: %s\n", resp.ID)
	}
}

// Example4_InteractiveMessage demonstra como criar mensagem interativa com NativeFlow
func Example4_InteractiveMessage(cli *whatsmeow.Client, targetJID types.JID) {
	// Criar header
	header := &waE2E.InteractiveMessage_Header{
		Title: proto.String("🎉 Promoção Especial"),
	}

	// Criar body
	body := &waE2E.InteractiveMessage_Body{
		Text: proto.String("Aproveite 20% de desconto em todos os produtos!"),
	}

	// Criar footer
	footer := &waE2E.InteractiveMessage_Footer{
		Text: proto.String("Válido até 31/12/2024"),
	}

	// Criar NativeFlowMessage com botões
	nativeFlow := &waE2E.InteractiveMessage_NativeFlowMessage{
		Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			{
				Name:             proto.String("cta_url"),
				ButtonParamsJSON: proto.String(`{"display_text":"Ver Produtos","url":"https://loja.exemplo.com"}`),
			},
			{
				Name:             proto.String("quick_reply"),
				ButtonParamsJSON: proto.String(`{"display_text":"Mais Informações","id":"more_info"}`),
			},
		},
		MessageParamsJSON: proto.String("{}"),
		MessageVersion:    proto.Int32(3),
	}

	// Construir mensagem
	msg := cli.BuildInteractiveMessage(header, body, footer, nativeFlow)

	// Enviar mensagem
	resp, err := cli.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		log.Printf("Erro ao enviar mensagem interativa: %v\n", err)
	} else {
		fmt.Printf("Mensagem interativa enviada! ID: %s\n", resp.ID)
	}
}

// Example5_Carousel demonstra como criar um carrossel de cards
func Example5_Carousel(cli *whatsmeow.Client, targetJID types.JID) {
	// Criar cards do carrossel
	cards := []*waE2E.InteractiveMessage{
		{
			Header: &waE2E.InteractiveMessage_Header{
				Title: proto.String("📱 Smartphone X"),
				Media: &waE2E.InteractiveMessage_Header_ImageMessage{
					ImageMessage: &waE2E.ImageMessage{
						// Aqui você pode adicionar uma imagem
						Caption: proto.String("Último modelo"),
					},
				},
			},
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String("Tela 6.5\", 128GB, Câmera 48MP\nR$ 2.499,00"),
			},
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{
						Name:             proto.String("quick_reply"),
						ButtonParamsJSON: proto.String(`{"display_text":"Comprar","id":"buy_smartphone_x"}`),
					},
				},
			},
		},
		{
			Header: &waE2E.InteractiveMessage_Header{
				Title: proto.String("⌚ Smartwatch Y"),
			},
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String("Monitor cardíaco, GPS, À prova d'água\nR$ 899,00"),
			},
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{
						Name:             proto.String("quick_reply"),
						ButtonParamsJSON: proto.String(`{"display_text":"Comprar","id":"buy_smartwatch_y"}`),
					},
				},
			},
		},
		{
			Header: &waE2E.InteractiveMessage_Header{
				Title: proto.String("🎧 Fones Bluetooth Z"),
			},
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String("Cancelamento de ruído, 30h de bateria\nR$ 449,00"),
			},
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{
						Name:             proto.String("quick_reply"),
						ButtonParamsJSON: proto.String(`{"display_text":"Comprar","id":"buy_headphones_z"}`),
					},
				},
			},
		},
	}

	// Construir mensagem de carrossel
	msg := cli.BuildCarouselMessage(
		cards,
		waE2E.InteractiveMessage_CarouselMessage_HSCROLL_CARDS,
	)

	// Enviar mensagem
	resp, err := cli.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		log.Printf("Erro ao enviar carrossel: %v\n", err)
	} else {
		fmt.Printf("Carrossel enviado! ID: %s\n", resp.ID)
	}
}

func main() {
	// NOTA: Este é apenas um exemplo de uso.
	// Para usar em produção, você precisa:
	// 1. Configurar a autenticação do WhatsApp
	// 2. Conectar o cliente
	// 3. Obter o JID do destinatário

	fmt.Println("Exemplos de uso das funções de mensagens interativas do Whatsmeow")
	fmt.Println("Veja o código-fonte para detalhes de implementação")
}

<?php
/**
 * Plugin Name: Digitaal GemeenGoed — projectvoorstellen
 * Description: Een lid stelt op de site een nieuw project voor (shortcode [dg_project_starten]). Het voorstel wordt een issue in Redmine (project "projectvoorstellen"), toegewezen aan de kerngroep. WordPress bewaart niets: Redmine is de bron.
 *
 * Verloop: het formulier → issue in Redmine (status Nieuw, naar de groep
 * Netwerk: Kerngroep) → de kerngroep bespreekt en zet het op Goedgekeurd →
 * redmine/redmine_groepen.py maakt het project aan, met de indiener als
 * projectleider; Documenten maakt de ruimte. De mapnaam (identifier) ligt
 * daarna vast.
 *
 * WordPress praat met Redmine als het account "site" (DG_REDMINE_SITE_KEY):
 * geen beheerder, alleen voorstellen indienen en lezen. Zo geeft een lek in
 * WordPress geen beheerrechten in Redmine.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

const DG_PV_PROJECT = 'projectvoorstellen';
// Ook in redmine/redmine_groepen.py: namen die Documenten of Redmine zelf gebruikt.
const DG_PV_GERESERVEERD       = array( 'cooperatie', 'projectvoorstellen', 'nieuw', 'new', 'mijn', 'beheer' );
const DG_PV_GERESERVEERD_BEGIN = array( 'werkgroep-', 'mijn-', 'project-' );

/** Een verzoek aan Redmine als het account "site". Geeft [statuscode, data] terug. */
function dg_pv_redmine( string $methode, string $pad, ?array $lichaam = null ): array {
	$basis   = rtrim( (string) ( getenv( 'DG_REDMINE_INTERN' ) ?: '' ), '/' );
	$sleutel = (string) getenv( 'DG_REDMINE_SITE_KEY' );
	if ( '' === $basis || '' === $sleutel ) {
		return array( 0, null );
	}
	$args = array(
		'method'  => $methode,
		'timeout' => 10,
		'headers' => array( 'Accept' => 'application/json', 'X-Redmine-API-Key' => $sleutel ),
	);
	if ( null !== $lichaam ) {
		$args['headers']['Content-Type'] = 'application/json';
		$args['body']                    = wp_json_encode( $lichaam );
	}
	$antwoord = wp_remote_request( $basis . $pad, $args );
	if ( is_wp_error( $antwoord ) ) {
		return array( 0, null );
	}
	return array( (int) wp_remote_retrieve_response_code( $antwoord ), json_decode( wp_remote_retrieve_body( $antwoord ), true ) );
}

function dg_pv_publiek( string $pad = '' ): string {
	return rtrim( (string) ( getenv( 'DG_REDMINE_URL' ) ?: '' ), '/' ) . $pad;
}

/** Een voorstel voor de mapnaam: "Zaakgericht archiveren!" → "zaakgericht-archiveren". */
function dg_pv_mapnaam_van( string $naam ): string {
	$m = strtolower( remove_accents( $naam ) );
	$m = trim( (string) preg_replace( '/[^a-z0-9]+/', '-', $m ), '-' );
	$m = (string) preg_replace( '/^[^a-z]+/', '', $m );
	return rtrim( substr( $m, 0, 40 ), '-' );
}

/** Waarom deze mapnaam niet kan, of '' als hij kan. */
function dg_pv_mapnaam_fout( string $m ): string {
	if ( ! preg_match( '/^[a-z][a-z0-9-]{2,39}$/', $m ) ) {
		return 'De mapnaam is 3 tot 40 tekens: kleine letters, cijfers en streepjes, en begint met een letter.';
	}
	if ( in_array( $m, DG_PV_GERESERVEERD, true ) ) {
		return 'Die mapnaam gebruikt het platform zelf. Kies een andere.';
	}
	foreach ( DG_PV_GERESERVEERD_BEGIN as $begin ) {
		if ( str_starts_with( $m, $begin ) ) {
			return 'Een mapnaam mag niet beginnen met "' . $begin . '": dat gebruikt het platform zelf.';
		}
	}
	return '';
}

/** De tracker en de velden van het project Projectvoorstellen (ids), of null. */
function dg_pv_inrichting(): ?array {
	$bewaard = get_transient( 'dg_pv_inrichting' );
	if ( is_array( $bewaard ) ) {
		return $bewaard;
	}
	[ $code, $data ] = dg_pv_redmine( 'GET', '/projects/' . DG_PV_PROJECT . '.json?include=trackers,issue_custom_fields' );
	if ( 200 !== $code || empty( $data['project'] ) ) {
		return null;
	}
	$uit = array( 'tracker' => 0, 'velden' => array() );
	foreach ( (array) ( $data['project']['trackers'] ?? array() ) as $t ) {
		if ( 'Projectvoorstel' === ( $t['name'] ?? '' ) ) {
			$uit['tracker'] = (int) $t['id'];
		}
	}
	foreach ( (array) ( $data['project']['issue_custom_fields'] ?? array() ) as $v ) {
		$uit['velden'][ (string) $v['name'] ] = (int) $v['id'];
	}
	if ( ! $uit['tracker'] || empty( $uit['velden']['Mapnaam'] ) || empty( $uit['velden']['Indiener'] ) ) {
		return null;
	}
	set_transient( 'dg_pv_inrichting', $uit, HOUR_IN_SECONDS );
	return $uit;
}

/** Is de mapnaam al een project, of het voorstel van een open voorstel? */
function dg_pv_bezet( string $m ): ?bool {
	[ $code ] = dg_pv_redmine( 'GET', '/projects/' . rawurlencode( $m ) . '.json' );
	if ( 0 === $code || $code >= 500 ) {
		return null; // weten we niet
	}
	if ( 404 !== $code ) {
		return true; // bestaat (ook als het besloten is: 403)
	}
	[ $code, $data ] = dg_pv_redmine( 'GET', '/issues.json?project_id=' . DG_PV_PROJECT . '&status_id=open&limit=100' );
	if ( 200 !== $code ) {
		return null;
	}
	foreach ( (array) ( $data['issues'] ?? array() ) as $issue ) {
		foreach ( (array) ( $issue['custom_fields'] ?? array() ) as $v ) {
			if ( 'Mapnaam' === ( $v['name'] ?? '' ) && strtolower( trim( (string) ( $v['value'] ?? '' ) ) ) === $m ) {
				return true;
			}
		}
	}
	return false;
}

// ---------------------------------------------------------------------------
// De pagina: [dg_project_starten]
// ---------------------------------------------------------------------------

add_shortcode( 'dg_project_starten', 'dg_project_starten' );

function dg_project_starten(): string {
	$html = '<div class="dg-pv" id="voorstel">';
	// phpcs:disable WordPress.Security.NonceVerification.Recommended -- alleen tonen.
	$soort = sanitize_key( wp_unslash( $_GET['dg_pv'] ?? '' ) );
	$nr    = (int) ( $_GET['nr'] ?? 0 );
	// phpcs:enable
	if ( 'ok' === $soort && $nr ) {
		return $html . dg_pv_ingediend( $nr ) . '</div>';
	}
	if ( ! is_user_logged_in() ) {
		return $html . '<p class="dg-cf-uitleg"><a href="' . esc_url( wp_login_url( get_permalink() . '#voorstel' ) ) . '">Log in</a> om een project voor te stellen. Nog geen lid? <a href="' . esc_url( home_url( '/lid-worden/' ) ) . '">Word lid</a>; dat is gratis.</p></div>';
	}
	if ( ! dg_pv_inrichting() ) {
		return $html . '<p class="dg-cf-melding dg-cf-melding-fout" role="status">Het formulier is even niet beschikbaar. Probeer het later opnieuw, of start een onderwerp in het forum.</p></div>';
	}
	$oud = (array) get_transient( 'dg_pv_invoer_' . get_current_user_id() );
	if ( 'fout' === $soort && ! empty( $oud['fout'] ) ) {
		$html .= '<p class="dg-cf-melding dg-cf-melding-fout" role="alert">' . esc_html( $oud['fout'] ) . '</p>';
	}
	$w = static fn( string $k ): string => esc_attr( (string) ( $oud[ $k ] ?? '' ) );
	$t = static fn( string $k ): string => esc_textarea( (string) ( $oud[ $k ] ?? '' ) );

	$html .= '<form class="dg-pv-form" method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">'
		. wp_nonce_field( 'dg_pv', '_wpnonce', true, false )
		. '<input type="hidden" name="action" value="dg_projectvoorstel">'
		. '<label>Naam van het project<input type="text" name="naam" id="dg-pv-naam" maxlength="120" required value="' . $w( 'naam' ) . '" placeholder="Bijvoorbeeld: Zaakgericht archiveren"></label>'
		. '<label>Wat wil je bereiken?<textarea name="doel" rows="4" maxlength="4000" required placeholder="Het doel in een paar zinnen: wat is er straks anders?">' . $t( 'doel' ) . '</textarea></label>'
		. '<label>Waarom nu? Welk probleem lost het op?<textarea name="aanleiding" rows="4" maxlength="4000" required>' . $t( 'aanleiding' ) . '</textarea></label>'
		. '<label>Wie doen er (misschien) mee?<input type="text" name="gemeenten" maxlength="250" value="' . $w( 'gemeenten' ) . '" placeholder="Gemeenten en organisaties, als je die al weet"></label>'
		. '<label>Wat is de eerste stap, en wat heb je nodig?<textarea name="eerste_stap" rows="3" maxlength="4000">' . $t( 'eerste_stap' ) . '</textarea></label>'
		. '<div class="dg-pv-vast">'
		. '<label>Mapnaam<input type="text" name="mapnaam" id="dg-pv-mapnaam" maxlength="40" required pattern="[a-z][a-z0-9\-]{2,39}" value="' . $w( 'mapnaam' ) . '" spellcheck="false" autocapitalize="off"></label>'
		. '<p><strong>Let op: de mapnaam ligt vast zodra het project start.</strong> Hij wordt het adres van het project in Redmine (<code>' . esc_html( wp_parse_url( dg_pv_publiek(), PHP_URL_HOST ) ?: 'projecten' ) . '/projects/<span data-dg-pv-voorbeeld>mapnaam</span></code>) en de naam van de map in Documenten, ook op een gekoppelde netwerkschijf. Zo blijven links en snelkoppelingen werken. De <em>naam</em> van het project kun je later wel veranderen.</p>'
		. '<p class="dg-pv-regels">Kleine letters, cijfers en streepjes; 3 tot 40 tekens; begint met een letter. Kort en herkenbaar, zonder jaartal of versienummer.</p>'
		. '<label class="dg-cf-vink"><input type="checkbox" name="vast" value="1" required> Ik begrijp dat de mapnaam na de start niet meer verandert.</label>'
		. '</div>'
		. '<button type="submit" class="dg-db-knop">Voorstel indienen</button>'
		. '</form>'
		. '<script>(function(){var n=document.getElementById("dg-pv-naam"),m=document.getElementById("dg-pv-mapnaam"),v=document.querySelector("[data-dg-pv-voorbeeld]");if(!n||!m)return;var zelf=m.value!=="";'
		. 'function maak(s){return s.normalize("NFD").replace(/[̀-ͯ]/g,"").toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^[^a-z]+/,"").slice(0,40).replace(/-+$/,"");}'
		. 'function toon(){if(v)v.textContent=m.value||"mapnaam";}'
		. 'n.addEventListener("input",function(){if(!zelf){m.value=maak(n.value);toon();}});'
		. 'm.addEventListener("input",function(){zelf=m.value!=="";m.value=m.value.toLowerCase().replace(/[^a-z0-9-]/g,"");toon();});toon();})();</script>';
	return $html . '</div>';
}

/** Na het indienen: wat er nu gebeurt. */
function dg_pv_ingediend( int $nr ): string {
	$link = dg_pv_publiek( '/issues/' . $nr );
	return '<div class="dg-cf-melding" role="status"><p><strong>Je voorstel is ingediend (nummer ' . (int) $nr . ').</strong> Dank je.</p></div>'
		. '<h3>Wat er nu gebeurt</h3><ol class="dg-pv-stappen">'
		. '<li><strong>De kerngroep krijgt je voorstel</strong> als taak. Je ziet het in Redmine: <a href="' . esc_url( $link ) . '">voorstel #' . (int) $nr . '</a>. Reacties en vragen krijg je per mail; je kunt er daar op antwoorden.</li>'
		. '<li><strong>Andere leden kunnen reageren</strong> en laten weten dat ze mee willen doen. Wijs collega\'s gerust op de link.</li>'
		. '<li><strong>Bij goedkeuring start het project vanzelf</strong>, binnen een paar minuten: een project in Redmine met jou als projectleider, en een ruimte in Documenten. De mapnaam ligt dan vast.</li>'
		. '<li><strong>Daarna</strong> nodig je mensen uit, zet je de eerste taken in Redmine en de stukken in Documenten.</li>'
		. '</ol><p>Je eigen taken en die van je groepen vind je onder <a href="' . esc_url( dg_pv_taken_url() ) . '">Taken</a> in je menu.</p>';
}

/** Redmine: wat aan jou of aan een van je groepen is toegewezen en nog open staat. */
function dg_pv_taken_url(): string {
	return dg_pv_publiek( '/issues?set_filter=1&assigned_to_id=me&status_id=o&sort=updated_on:desc' );
}

// ---------------------------------------------------------------------------
// Het formulier verwerken
// ---------------------------------------------------------------------------

add_action(
	'admin_post_dg_projectvoorstel',
	static function () {
		$terug = remove_query_arg( array( 'dg_pv', 'nr' ), wp_get_referer() ?: home_url( '/projecten/project-starten/' ) );
		$terug = strtok( $terug, '#' );
		$uid   = get_current_user_id();
		$tekst = static fn( string $k, int $max ): string => mb_substr( trim( sanitize_textarea_field( wp_unslash( $_POST[ $k ] ?? '' ) ) ), 0, $max ); // phpcs:ignore WordPress.Security.NonceVerification.Missing -- hieronder gecontroleerd
		$invoer = array(
			'naam'        => mb_substr( trim( sanitize_text_field( wp_unslash( $_POST['naam'] ?? '' ) ) ), 0, 120 ), // phpcs:ignore WordPress.Security.NonceVerification.Missing
			'doel'        => $tekst( 'doel', 4000 ),
			'aanleiding'  => $tekst( 'aanleiding', 4000 ),
			'gemeenten'   => mb_substr( trim( sanitize_text_field( wp_unslash( $_POST['gemeenten'] ?? '' ) ) ), 0, 250 ), // phpcs:ignore WordPress.Security.NonceVerification.Missing
			'eerste_stap' => $tekst( 'eerste_stap', 4000 ),
			'mapnaam'     => strtolower( trim( sanitize_text_field( wp_unslash( $_POST['mapnaam'] ?? '' ) ) ) ), // phpcs:ignore WordPress.Security.NonceVerification.Missing
		);
		$fout = static function ( string $reden ) use ( $uid, $invoer, $terug ): void {
			set_transient( 'dg_pv_invoer_' . $uid, $invoer + array( 'fout' => $reden ), HOUR_IN_SECONDS );
			wp_safe_redirect( add_query_arg( 'dg_pv', 'fout', $terug ) . '#voorstel' );
			exit;
		};
		if ( ! $uid || ! check_admin_referer( 'dg_pv' ) ) {
			wp_safe_redirect( $terug );
			exit;
		}
		if ( '' === $invoer['naam'] || '' === $invoer['doel'] || '' === $invoer['aanleiding'] ) {
			$fout( 'Vul de naam, het doel en de aanleiding in.' );
		}
		if ( empty( $_POST['vast'] ) ) {
			$fout( 'Vink aan dat je begrijpt dat de mapnaam na de start vastligt.' );
		}
		$reden = dg_pv_mapnaam_fout( $invoer['mapnaam'] );
		if ( $reden ) {
			$fout( $reden );
		}
		if ( get_transient( 'dg_pv_net_' . $uid ) ) {
			$fout( 'Je hebt net een voorstel ingediend. Wacht een minuut voordat je er nog een indient.' );
		}
		$inrichting = dg_pv_inrichting();
		$bezet      = dg_pv_bezet( $invoer['mapnaam'] );
		if ( ! $inrichting || null === $bezet ) {
			$fout( 'Redmine is even niet bereikbaar. Probeer het over een paar minuten opnieuw.' );
		}
		if ( $bezet ) {
			$fout( 'De mapnaam "' . $invoer['mapnaam'] . '" is al in gebruik, door een project of een ander voorstel. Kies een andere.' );
		}

		$gebruiker = wp_get_current_user();
		$org       = (string) get_user_meta( $uid, 'dg_organisatie', true );
		$delen     = array(
			'**Wat wil je bereiken?**' . "\n\n" . $invoer['doel'],
			'**Waarom nu? Welk probleem lost het op?**' . "\n\n" . $invoer['aanleiding'],
		);
		if ( '' !== $invoer['eerste_stap'] ) {
			$delen[] = '**Eerste stap en wat nodig is**' . "\n\n" . $invoer['eerste_stap'];
		}
		$delen[] = '_Ingediend door ' . $gebruiker->display_name . ( $org ? ' (' . $org . ')' : '' ) . ' via de site._';
		$velden  = array(
			array( 'id' => $inrichting['velden']['Mapnaam'], 'value' => $invoer['mapnaam'] ),
			array( 'id' => $inrichting['velden']['Indiener'], 'value' => strtolower( $gebruiker->user_email ) ),
		);
		if ( ! empty( $inrichting['velden']['Gemeenten'] ) ) {
			$velden[] = array( 'id' => $inrichting['velden']['Gemeenten'], 'value' => $invoer['gemeenten'] );
		}
		[ $code, $data ] = dg_pv_redmine(
			'POST',
			'/issues.json',
			array(
				'issue' => array(
					'project_id'    => DG_PV_PROJECT,
					'tracker_id'    => $inrichting['tracker'],
					'subject'       => $invoer['naam'],
					'description'   => implode( "\n\n", $delen ),
					'custom_fields' => $velden,
				),
			)
		);
		if ( 201 !== $code || empty( $data['issue']['id'] ) ) {
			delete_transient( 'dg_pv_inrichting' );
			$fout( 'Het voorstel kon niet worden opgeslagen' . ( ! empty( $data['errors'] ) ? ': ' . implode( '; ', (array) $data['errors'] ) : '' ) . '. Probeer het opnieuw.' );
		}
		delete_transient( 'dg_pv_invoer_' . $uid );
		set_transient( 'dg_pv_net_' . $uid, 1, MINUTE_IN_SECONDS );
		wp_safe_redirect( add_query_arg( array( 'dg_pv' => 'ok', 'nr' => (int) $data['issue']['id'] ), $terug ) . '#voorstel' );
		exit;
	}
);
